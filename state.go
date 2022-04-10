package maquina

import "context"

// NewState instantiates a state with a label for tracking and tracing.
func NewState[T input](label string, defaultInput T) *State[T] {
	if label == "" {
		panic("label cannot be empty")
	}
	return &State[T]{
		label: label,
	}
}

// Permit registers a state transition from receiver s to dst when Trigger t is
// invoked given the guard clauses return true. If any of the guad clauses return
// false the state transition is aborted and the Fire() attempt by the state machine
// returns an error.
func (s *State[T]) Permit(t Trigger, dst *State[T], guards ...GuardClause[T]) {
	mustNotBeWildcard(t)
	s.transitions = append(s.transitions, Transition[T]{
		Src: s, Dst: dst, Trigger: t, guards: guards,
	})
}

func (s *State[T]) OnEntryFrom(t Trigger, f func(ctx context.Context, input T)) {
	mustNotBeWildcard(t)
	s.onEntryInternal(t, f)
}

func (s *State[T]) OnEntry(f func(ctx context.Context, input T)) {
	s.onEntryInternal(triggerWildcard, f)
}

func (s *State[T]) OnExitThrough(t Trigger, f func(ctx context.Context, input T)) {
	mustNotBeWildcard(t)
	s.onExitInternal(t, f)
}

func (s *State[T]) OnExit(f func(ctx context.Context, input T)) {
	s.onExitInternal(triggerWildcard, f)
}

func (s *State[T]) OnReentry(f func(ctx context.Context, input T)) {
	s.onReentryInternal(triggerWildcard, f)
}

func (s *State[T]) OnReentryFrom(t Trigger, f func(ctx context.Context, input T)) {
	mustNotBeWildcard(t)
	s.onReentryInternal(t, f)
}

func (s *State[T]) onExitInternal(t Trigger, f FringeFunc[T]) {
	s.exitFuncs = append(s.exitFuncs, triggeredFunc[T]{
		f: f,
		t: t,
	})
}

func (s *State[T]) onReentryInternal(t Trigger, f FringeFunc[T]) {
	s.reentryFuncs = append(s.reentryFuncs, triggeredFunc[T]{
		f: f,
		t: t,
	})
}

func (s *State[T]) onEntryInternal(t Trigger, f FringeFunc[T]) {
	s.entryFuncs = append(s.entryFuncs, triggeredFunc[T]{
		f: f,
		t: t,
	})
}

func mustNotBeWildcard(t Trigger) {
	if t == triggerWildcard {
		panic("trigger \"" + string(triggerWildcard) + "\" reserved for wildcard")
	}
}
