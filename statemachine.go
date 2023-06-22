package maquina

import (
	"context"
)

// StateMachine handles state transitioning control flow. Is not yet concurrency safe
// nor may it ever be concurrent safe since this would probably be implemented in
// a type that embeds StateMachine.
type StateMachine[T input] struct {
	actual             *State[T]
	onUnhandledTrigger func(s *State[T], t Trigger) error
	onTransitioning    FringeCallback[T]
	onTransitioned     FringeCallback[T]
}

// NewStateMachine returns a StateMachine with initial State s.
func NewStateMachine[T input](s *State[T]) *StateMachine[T] {
	if s == nil {
		panic("nil initial state")
	}
	return &StateMachine[T]{
		actual: s,
	}
}

// State returns the current state.
func (sm *StateMachine[T]) State() *State[T] {
	return sm.actual
}

// StateLabel returns the current state label. Is shorthand for sm.State().Label().
// Is provided for convenience as a method to allow allow construction
// of state machine interface types with no type parameters.
func (sm *StateMachine[T]) StateLabel() string { return sm.State().Label() }

// FireBg fires the state transition corresponding to the trigger T with
// context.Background().
//
// FireBg returns an error in the following cases:
//   - A guard clause fails to validate (returns GuardClauseError).
//   - OnUnhandledTrigger registered callback catches an unhandled trigger and returns an error.
//
// FireBg panics if there is no registered trigger on the current state and the
// OnUnhandledTrigger callback has not been set.
func (sm *StateMachine[T]) FireBg(t Trigger, input T) error {
	return sm.Fire(context.Background(), t, input)
}

// Fire fires the state transition corresponding to the trigger T.
//
// Fire returns an error in the following cases:
//   - ctx.Err() != nil (cancelled context) for the case where the context is cancelled
//     before the exit/reentry functions are run.
//   - A guard clause fails to validate (returns GuardClauseError).
//   - OnUnhandledTrigger registered callback catches an unhandled trigger and returns an error.
//
// Fire panics if there is no registered trigger on the current state and the
// OnUnhandledTrigger callback has not been set.
func (sm *StateMachine[T]) Fire(ctx context.Context, t Trigger, input T) error {
	if t == triggerWildcard {
		panic("cannot fire wildcard trigger") // Panic since this would imply a bug in the code.
	}
	transition := sm.actual.getTransition(t)
	if transition == nil {
		if sm.onUnhandledTrigger != nil {
			return sm.onUnhandledTrigger(sm.actual, t)
		}
		panic("trigger " + t.Quote() + " not handled for state " + sm.actual.String())
	}
	tr := *transition
	if sm.onTransitioning.cb != nil {
		sm.onTransitioning.cb(ctx, tr, input)
	}
	err := fire(ctx, tr, input)
	if err != nil {
		// an error here usually means a guard clause did not validate.
		// or context.Context was cancelled (ctx.Err() != nil)
		return err
	}
	sm.actual = transition.Dst
	if sm.onTransitioned.cb != nil {
		sm.onTransitioned.cb(ctx, tr, input)
	}
	return nil
}

// TriggersPermitted returns triggers which are permitted for
// the current State given input and ctx Context by calling the guard clauses with input.
// A Trigger transition is permitted if all guard clauses return true.
func (sm *StateMachine[T]) TriggersPermitted(ctx context.Context, input T) []Trigger {
	var permitted []Trigger
	for _, transition := range sm.actual.transitions {
		if err := transition.isPermitted(ctx, input); err == nil {
			permitted = append(permitted, transition.Trigger)
		}
	}
	return permitted
}

// TriggersAvailable returns all triggers registered for the current State.
// Firing any of these triggers may fail if a guard clause returns false.
func (sm *StateMachine[T]) TriggersAvailable() []Trigger {
	var available []Trigger
	for _, transition := range sm.actual.transitions {
		available = append(available, transition.Trigger)
	}
	return available
}

// OnUnhandledTrigger registeres the callback for when a trigger with no
// transition is encountered for the StateMachine's current state.
// It replaces the callback set by a previous call to OnUnhandledTrigger.
func (sm *StateMachine[T]) OnUnhandledTrigger(f func(current *State[T], t Trigger) error) {
	sm.onUnhandledTrigger = f
}

// OnTransitioning registers the callback which is invoked when transitioning commences.
// It replaces the callback set by a previous call to OnTransitioning.
// It is the first callback executed when transitioning, preceding any guard clauses,
// exiting and entering callbacks.
func (sm *StateMachine[T]) OnTransitioning(fcb FringeCallback[T]) {
	sm.onTransitioning = fcb
}

// OnTransitioned registers the callback which is invoked when transition finalizes.
// It replaces the callback set by a previous call to OnTransitioned.
// It is called after all guard clauses, exiting and entering callbacks, and
// after the state machine has had its actual state changed to the destination state.
// It is not called if the transition fails and the new destination state is not set.
func (sm *StateMachine[T]) OnTransitioned(fcb FringeCallback[T]) {
	sm.onTransitioned = fcb
}

// AlwaysPermit registers a trigger which is always permitted for the current state.
// Triggers set on a state take precedence over an always permitted trigger.
// It panics if trigger is the wildcard trigger or if dst is nil.
func (sm *StateMachine[T]) AlwaysPermit(trigger Trigger, dst *State[T], guards ...GuardClause[T]) {
	trigger.mustNotBeWildcard()
	if dst == nil {
		panic("nil destination state")
	}
	transition := Transition[T]{
		Trigger: trigger,
		Dst:     dst,
		guards:  guards,
	}
	// To maintain consistency of our state machine we add the always permitted
	// transition to all states in our tree without the transition.
	WalkStates(sm.actual, func(s *State[T]) (err error) {
		if !s.hasTransition(trigger) {
			transitionWithSrc := transition
			transitionWithSrc.Src = s
			s.transitions = append(s.transitions, transitionWithSrc)
		}
		return nil
	})
	// add the transition to the destination state if it does not already have it.
	if !dst.hasTransition(trigger) {
		transition.Src = dst
		dst.transitions = append(dst.transitions, transition)
	}
}

// StateIsSource returns true if the current state is a source state, that is to say
// if it has no incoming transitions. throughout the whole state machine.
// Once a source state is transitioned out of it cannot be transitioned back to.
func (sm *StateMachine[T]) StateIsSource() bool {
	currentState := sm.State()
	isSource := true
	WalkStates(currentState, func(s *State[T]) error {
		for _, tr := range s.transitions {
			if statesEqual(currentState, tr.Dst) {
				isSource = false
				return errTriggerWildcardNotAllowed // Break out of WalkStates.
			}
		}
		return nil
	})
	return isSource
}

// StateIsSink returns true if the current state is a sink state, that is to say
// it has no transitions to states other than itself.
func (sm *StateMachine[T]) StateIsSink() bool {
	return sm.State().isSink()
}
