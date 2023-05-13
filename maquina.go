package maquina

import (
	"context"
	"errors"
)

// input is an alias for any for the time being. Will probably remain as such
// unless a good reason to change it arises.
type input = any

// Trigger represents the transition from one state to another.
type Trigger string

// State basic functional unit of a finite state machine.
type State[T input] struct {
	label        string
	transitions  []Transition[T]
	exitFuncs    []triggeredFunc[T]
	entryFuncs   []triggeredFunc[T]
	reentryFuncs []triggeredFunc[T]
	defaultInput T
}

// Transition contains information regarding a triggered transition from one state
// to another. It can represent an reentry transition.
type Transition[T input] struct {
	Src     *State[T]
	Dst     *State[T]
	Trigger Trigger
	guards  []GuardClause[T]
}

// HasGuards returns true if the transition has any guard clauses.
func (t Transition[T]) HasGuards() bool { return len(t.guards) > 0 }

// IsReentry checks if the transition is a reentry transition.
func (t Transition[T]) IsReentry() bool { return statesEqual(t.Src, t.Dst) }

// Guards returns a copy of the guard clauses for the transition.
func (t Transition[T]) Guards() []GuardClause[T] {
	return append([]GuardClause[T]{}, t.guards...) // clone guard clauses
}

// GuardClause represents a condition that must be met for a state
// transition to complete succesfully. If a GuardClause returns false
// during a transition the transition halts and the state remains as before.
type GuardClause[T input] struct {
	label string
	guard func(ctx context.Context, input T) error
}

// String returns the label with which gc was created.
func (gc GuardClause[T]) String() string { return gc.label }

// NewGuard instantiates a new GuardClause with a label and a guard function.
func NewGuard[T input](label string, guard func(ctx context.Context, input T) error) GuardClause[T] {
	return GuardClause[T]{label: label, guard: guard}
}

type triggeredFunc[T input] struct {
	t Trigger
	f fringeFunc[T]
}

type fringeFunc[T input] func(ctx context.Context, input T)

// String returns the trigger string with which it was created.
func (t Trigger) String() string { return string(t) }

// Quote returns the trigger string with which it was create enclosed in double quotes.
func (t Trigger) Quote() string { return "\"" + t.String() + "\"" }

// triggerWildcard is a special trigger that matches any trigger during
// comparisons to check if a callback should be run. These include exit, reentry
// and entry callbacks. Wildcards are set internally by go-maquina for callbacks
// which should always run regardless of the transition.
var triggerWildcard Trigger = "*"

// triggersEqual checks if a trigger is equal to another trigger or the wildcard.
// Should only be used for checking if a callback should be run.
func triggersEqual(a, b Trigger) bool          { return a == b || a == triggerWildcard || b == triggerWildcard }
func statesEqual[T input](a, b *State[T]) bool { return a.label == b.label }

func (s *State[T]) exit(ctx context.Context, t Trigger, input T) {
	for i := 0; i < len(s.exitFuncs); i++ {
		if triggersEqual(s.exitFuncs[i].t, t) {
			s.exitFuncs[i].f(ctx, input)
		}
	}
}

func (s *State[T]) enter(ctx context.Context, t Trigger, input T) {
	for i := 0; i < len(s.entryFuncs); i++ {
		if triggersEqual(s.entryFuncs[i].t, t) {
			s.entryFuncs[i].f(ctx, input)
		}
	}
}

func (s *State[T]) reenter(ctx context.Context, t Trigger, input T) {
	for i := 0; i < len(s.reentryFuncs); i++ {
		if triggersEqual(s.reentryFuncs[i].t, t) {
			s.reentryFuncs[i].f(ctx, input)
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
	// TODO(soypat): Document how this panic is reached, if at all possible-- if not reachable state so.
	panic("could not find firing trigger " + t.Quote() + " registered with state " + s.label)
}

func (s *State[T]) getTransition(t Trigger) *Transition[T] {
	for i := 0; i < len(s.transitions); i++ {
		if t == s.transitions[i].Trigger {
			return &s.transitions[i]
		}
	}
	return nil
}

type guardClauseError struct {
	Label string
	Err   error
}

func (g *guardClauseError) Error() string {
	return "guard clause \"" + g.Label + "\" failed: " + g.Err.Error()
}
func (g *guardClauseError) Unwrap() error { return g.Err }

func (tr Transition[T]) fire(ctx context.Context, t Trigger, input T) error {
	if err := tr.isPermitted(ctx, input); err != nil {
		return err
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
	for i := 0; i < len(tr.guards); i++ {
		if err := tr.guards[i].guard(ctx, input); err != nil {
			return &guardClauseError{Err: err, Label: tr.guards[i].label}
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

// walkStateTransitions recurses down the state tree in a breadth first search for
// all transitions in what would be a state machine starting with the argument state.
// It calls fn on every transition it finds. If fn returns an error, the walk is aborted
// and the error is returned.
func walkStateTransitions[T input](start *State[T], fn func(tr Transition[T]) error) error {
	if start == nil {
		return errors.New("start state is nil")
	}
	visited := make(map[string]struct{})
	return walkTransitions(start, fn, visited)
}

func walkTransitions[T input](src *State[T], fn func(tr Transition[T]) error, visited map[string]struct{}) error {
	if _, ok := visited[src.label]; ok {
		return nil
	}
	visited[src.label] = struct{}{}
	for i := 0; i < len(src.transitions); i++ {
		if !statesEqual(src, src.transitions[i].Src) {
			panic("state's transition source not match self: " + src.String() + " != " + src.transitions[i].Src.String())
		}
		err := fn(src.transitions[i])
		if err != nil {
			return err
		}
		err = walkTransitions(src.transitions[i].Dst, fn, visited)
		if err != nil {
			return err
		}
	}
	return nil
}
