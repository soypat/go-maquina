package maquina

import (
	"context"
	"errors"
)

// NewState instantiates a state with a label for tracking and tracing.
// The type parameter T will be the argument received by entry, exit,
// reentry and guard clause callbacks during state transitions.
//
// defaultInput is still unused.
func NewState[T input](label string, defaultInput T) *State[T] {
	if label == "" {
		panic("label cannot be empty")
	}
	return &State[T]{
		label: label,
	}
}

// Permit registers a state transition from receiver s to dst when Trigger t is
// invoked given the guard clauses return true. If any of the guard clauses return
// false the state transition is aborted and the Fire() attempt by the state machine
// returns an error.
func (s *State[T]) Permit(t Trigger, dst *State[T], guards ...GuardClause[T]) {
	s.mustValidate(t)
	s.transitions = append(s.transitions, Transition[T]{
		Src: s, Dst: dst, Trigger: t, guards: guards,
	})
}

// OnEntryFrom registers a callback that executes on entering State s
// through Trigger t. Does not execute on reentry.
func (s *State[T]) OnEntryFrom(t Trigger, f func(ctx context.Context, input T)) {
	s.mustValidate(t)
	s.onEntryInternal(t, f)
}

// OnEntry registers a callback that executes on entering State s.
// Does not execute on reentry.
func (s *State[T]) OnEntry(f func(ctx context.Context, input T)) {
	s.onEntryInternal(triggerWildcard, f)
}

// OnExitThrough registers a callback that executes on exiting State s
// through Trigger t. Does not execute on reentry.
func (s *State[T]) OnExitThrough(t Trigger, f func(ctx context.Context, input T)) {
	s.mustValidate(t)
	s.onExitInternal(t, f)
}

// OnExit registers a callback that executes on exiting State s.
// Does not execute on reentry.
func (s *State[T]) OnExit(f func(ctx context.Context, input T)) {
	s.onExitInternal(triggerWildcard, f)
}

// OnReentry registers a callback that executes when reentering State s.
func (s *State[T]) OnReentry(f func(ctx context.Context, input T)) {
	s.onReentryInternal(triggerWildcard, f)
}

// OnReentryFrom registers a callback that executes when reentering State s through Trigger t.
func (s *State[T]) OnReentryFrom(t Trigger, f func(ctx context.Context, input T)) {
	s.mustValidate(t)
	s.onReentryInternal(t, f)
}

func (s *State[T]) hasTransition(t Trigger) bool {
	for i := 0; i < len(s.transitions); i++ {
		if s.transitions[i].Trigger == t {
			return true
		}
	}
	return false
}

func (s *State[T]) isSink() bool {
	for i := 0; i < len(s.transitions); i++ {
		if !statesEqual(s, s.transitions[i].Dst) {
			return false
		}
	}
	return true
}

func (s *State[T]) onExitInternal(t Trigger, f fringeFunc[T]) {
	if f == nil {
		panic("onExit function cannot be nil")
	}
	s.exitFuncs = append(s.exitFuncs, triggeredFunc[T]{
		f: f,
		t: t,
	})
}

func (s *State[T]) onReentryInternal(t Trigger, f fringeFunc[T]) {
	if f == nil {
		panic("onReentry function cannot be nil")
	}
	s.reentryFuncs = append(s.reentryFuncs, triggeredFunc[T]{
		f: f,
		t: t,
	})
}

func (s *State[T]) onEntryInternal(t Trigger, f fringeFunc[T]) {
	if f == nil {
		panic("onEntry function cannot be nil")
	}
	s.entryFuncs = append(s.entryFuncs, triggeredFunc[T]{
		f: f,
		t: t,
	})
}

var errTriggerWildcardNotAllowed = errors.New("trigger " + triggerWildcard.Quote() + " reserved for internal use (wildcard)")

func (s *State[T]) mustValidate(t Trigger) {
	switch t {
	case "":
		panic("trigger must not be empty string")
	case triggerWildcard:
		panic(errTriggerWildcardNotAllowed)
	}
	existingTransition := s.getTransition(t)
	if existingTransition != nil {
		panic("trigger " + t.Quote() + " already registed as transition: " + existingTransition.String())
	}
}
