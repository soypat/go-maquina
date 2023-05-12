package maquina

import "context"

// StateMachine handles state transitioning control flow. Is not yet concurrency safe.
type StateMachine[T input] struct {
	// will be used in future for concurrency enabling features
	fireOp             uint64
	actual             *State[T]
	onUnhandledTrigger func(s *State[T], t Trigger) error
	onTransitioning    func(tr Transition[T])
	onTransitioned     func(tr Transition[T])
}

// NewStateMachine returns a StateMachine with initial State s.
func NewStateMachine[T input](s *State[T]) *StateMachine[T] {
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
	transition := sm.actual.getTransition(t)
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

// OnUnhandledTrigger registeres a callback for when a trigger with no transition is encountered for the
// StateMachine's current state.
func (sm *StateMachine[T]) OnUnhandledTrigger(f func(current *State[T], t Trigger) error) {
	sm.onUnhandledTrigger = f
}

// OnTransitioning registers a callback which is invoked when transitioning commences.
func (sm *StateMachine[T]) OnTransitioning(f func(s Transition[T])) {
	sm.onTransitioning = f
}

// OnTransitioned registers a callback which is invoked when transition finalizes.
func (sm *StateMachine[T]) OnTransitioned(f func(s Transition[T])) {
	sm.onTransitioned = f
}
