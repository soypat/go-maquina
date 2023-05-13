package maquina

import (
	"context"
	"errors"
)

// StateMachine handles state transitioning control flow. Is not yet concurrency safe.
type StateMachine[T input] struct {
	// will be used in future for concurrency enabling features
	fireOp             uint64
	actual             *State[T]
	alwaysPermitted    []Transition[T]
	onUnhandledTrigger func(s *State[T], t Trigger) error
	onTransitioning    func(tr Transition[T])
	onTransitioned     func(tr Transition[T])
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

// FireBg fires the state transition corresponding to the trigger T with
// context.Background().
//
// FireBg returns an error in the following cases:
//   - A guard clause fails to validate (returns wrapped error)
//   - OnUnhandledTrigger registered callback catches an unhandled trigger and returns an error.
func (sm *StateMachine[T]) FireBg(t Trigger, input T) error {
	return sm.Fire(context.Background(), t, input)
}

// Fire fires the state transition corresponding to the trigger T.
//
// Fire returns an error in the following cases:
//   - ctx.Err() != nil (cancelled context). Fire returns ctx.Err() in this case.
//   - A guard clause fails to validate (returns wrapped error)
//   - OnUnhandledTrigger registered callback catches an unhandled trigger and returns an error.
func (sm *StateMachine[T]) Fire(ctx context.Context, t Trigger, input T) error {
	if t == triggerWildcard {
		panic("cannot fire wildcard trigger") // Panic since this would imply a bug in the code.
	}
	transition := sm.actual.getTransition(t)
	if transition == nil {
		// check if the trigger is always permitted
		for i := 0; i < len(sm.alwaysPermitted); i++ {
			if t == sm.alwaysPermitted[i].Trigger {
				transition = &sm.alwaysPermitted[i]
			}
		}
	}
	if transition == nil {
		if sm.onUnhandledTrigger != nil {
			return sm.onUnhandledTrigger(sm.actual, t)
		}
		panic("trigger " + t.Quote() + " not handled for state " + sm.actual.String())
	}
	if sm.onTransitioning != nil {
		sm.onTransitioning(*transition)
	}
	err := sm.actual.fire(ctx, t, input)
	if err != nil {
		// an error here usually means a guard clause did not validate.
		// or context.Context was cancelled (ctx.Err() != nil)
		return err
	}
	if sm.onTransitioned != nil {
		sm.onTransitioned(*transition)
	}
	sm.actual = transition.Dst
	return nil
}

// PermittedTriggers returns triggers which are permitted for
// the current State given input and ctx Context by calling the guard clauses with input.
// A Trigger transition is permitted if all guard clauses return true.
func (sm *StateMachine[T]) PermittedTriggers(ctx context.Context, input T) []Trigger {
	var permitted []Trigger
	for _, transition := range sm.actual.transitions {
		if err := transition.isPermitted(ctx, input); err == nil {
			permitted = append(permitted, transition.Trigger)
		}
	}
	return permitted
}

// AvailableTriggers returns all triggers registered for the current State.
// Firing any of these triggers may fail if a guard clause returns false.
func (sm *StateMachine[T]) AvailableTriggers() []Trigger {
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
func (sm *StateMachine[T]) OnTransitioning(f func(s Transition[T])) {
	sm.onTransitioning = f
}

// OnTransitioned registers the callback which is invoked when transition finalizes.
// It replaces the callback set by a previous call to OnTransitioned.
func (sm *StateMachine[T]) OnTransitioned(f func(s Transition[T])) {
	sm.onTransitioned = f
}

var errExitWalk = errors.New("exit walk")

// AlwaysPermit registers a trigger which is always permitted for the current state.
// Triggers set on a state take precedence over an always permitted trigger.
// It panics if trigger is the wildcard trigger or if dst is nil.
func (sm *StateMachine[T]) AlwaysPermit(trigger Trigger, dst *State[T], guards ...GuardClause[T]) {
	if trigger == triggerWildcard {
		panic(errTriggerWildcardNotAllowed)
	} else if dst == nil {
		panic("nil destination state")
	}
	transition := Transition[T]{
		Trigger: trigger,
		Dst:     dst,
		guards:  guards,
	}
	// To maintain consistency of our state machine and ensure that the dst
	// state is reachable within the state tree, we add the always permitted
	// transition to a state in our tree without the transition.
	walkStateTransitions(sm.actual, func(tr Transition[T]) (err error) {
		if !tr.Src.hasTransition(trigger) {
			transitionWithSrc := transition
			transitionWithSrc.Src = tr.Src
			tr.Src.transitions = append(tr.Src.transitions, transitionWithSrc)
			err = errExitWalk
		} else if !tr.Dst.hasTransition(trigger) {
			transitionWithSrc := transition
			transitionWithSrc.Src = tr.Dst
			tr.Dst.transitions = append(tr.Dst.transitions, transitionWithSrc)
			err = errExitWalk
		}
		return err
	})
	sm.alwaysPermitted = append(sm.alwaysPermitted, transition)
}
