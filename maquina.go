package maquina

import (
	"context"
)

type input = any

// Trigger represents the transition from one state to another.
type Trigger string

// State basic functional unit of a finite state machine.
type State[T input] struct {
	defaultInput T
	label        string
	transitions  []Transition[T]
	exitFuncs    []triggeredFunc[T]
	entryFuncs   []triggeredFunc[T]
	reentryFuncs []triggeredFunc[T]
}

// Transition contains information regarding a triggered transition from one state
// to another. It can represent an reentry transition.
type Transition[T input] struct {
	Src     *State[T]
	Dst     *State[T]
	Trigger Trigger
	guards  []GuardClause[T]
}

// GuardClause represents a condition that must be met for a state
// transition to complete succesfully. If a GuardClause returns false
// during a transition the transition halts and the state remains as before.
type GuardClause[T input] func(ctx context.Context, input T) error

type triggeredFunc[T input] struct {
	t Trigger
	f fringeFunc[T]
}

type fringeFunc[T input] func(ctx context.Context, input T)

// String returns the trigger string with which it was created.
func (t Trigger) String() string { return string(t) }

// Quote returns the trigger string with which it was create enclosed in double quotes.
func (t Trigger) Quote() string { return "\"" + t.String() + "\"" }

var triggerWildcard Trigger = "*"

func triggersEqual(a, b Trigger) bool          { return a == b || a == triggerWildcard || b == triggerWildcard }
func statesEqual[T input](a, b *State[T]) bool { return a.label == b.label }

func (s *State[T]) exit(ctx context.Context, t Trigger, input T) {
	for _, tf := range s.exitFuncs {
		if triggersEqual(tf.t, t) {
			tf.f(ctx, input)
		}
	}
}

func (s *State[T]) enter(ctx context.Context, t Trigger, input T) {
	for _, tf := range s.entryFuncs {
		if triggersEqual(tf.t, t) {
			tf.f(ctx, input)
		}
	}
}

func (s *State[T]) reenter(ctx context.Context, t Trigger, input T) {
	for _, tf := range s.reentryFuncs {
		if triggersEqual(tf.t, t) {
			tf.f(ctx, input)
		}
	}
}

// fire returns error if transition was unable to be completed
// in which case the state remains same as before.
//
// fire should panic if transition started, that is to say any exit
// or entry function was run and encountered an error since this would
// leave the state machine in an undefined state. Guard clauses should
// prevent this from happening.
func (s *State[T]) fire(ctx context.Context, t Trigger, input T) error {
	if transition := s.getTransition(t); transition != nil {
		return transition.fire(ctx, t, input)
	}
	panic("could not find firing trigger " + t.Quote() + " registered with state " + s.label)
}

func (s *State[T]) getTransition(t Trigger) *Transition[T] {
	for i := range s.transitions {
		if triggersEqual(t, s.transitions[i].Trigger) {
			return &s.transitions[i]
		}
	}
	return nil
}

type guardClauseError struct {
	Err error
}

func (g *guardClauseError) Error() string { return "guard clause failed: " + g.Err.Error() }
func (g *guardClauseError) Unwrap() error { return g.Err }

func (tr Transition[T]) fire(ctx context.Context, t Trigger, input T) error {
	if err := tr.isPermitted(ctx, input); err != nil {
		return &guardClauseError{Err: err}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if statesEqual(tr.Src, tr.Dst) {
		tr.Src.reenter(ctx, t, input)
		return nil
	}
	tr.Src.exit(ctx, t, input)
	tr.Dst.enter(ctx, t, input)
	return nil
}

func (tr Transition[T]) isPermitted(ctx context.Context, input T) error {
	for _, guard := range tr.guards {
		if err := guard(ctx, input); err != nil {
			return err
		}
	}
	return nil
}

// String returns a basic text-arrow representation of the transition.
func (tr Transition[T]) String() string {
	return tr.Src.label + " --" + tr.Trigger.String() + "-> " + tr.Dst.label
}

// String returns the label with which the State was initialized.
func (s State[T]) String() string { return s.label }
