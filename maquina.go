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
func fire[T input](ctx context.Context, tr *Transition[T], input T) error {
	if err := tr.isPermitted(ctx, input); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if statesEqual(tr.Src, tr.Dst) {
		tr.Src.reenter(ctx, tr.Trigger, input)
		return nil
	}
	tr.Src.exit(ctx, tr.Trigger, input)
	tr.Dst.enter(ctx, tr.Trigger, input)
	return nil
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
	str := tr.Src.label + " --" + tr.Trigger.String() + "-> " + tr.Dst.label
	for i := 0; i < len(tr.guards); i++ {
		str += " [" + tr.guards[i].String() + "] "
	}
	return str
}

// String returns the label with which the State was initialized.
func (s State[T]) String() (str string) {
	str += s.label + ":\n"
	for i := 0; i < len(s.transitions); i++ {
		str += "\t" + s.transitions[i].String() + "\n"
	}
	return str
}

// WalkStates recurses down the state tree in a depth first search for
// all unique states in what would be a state machine starting with the argument state.
// It calls fn on every new state it finds. If fn returns an error, the walk is aborted
// and the error is returned.
func WalkStates[T input](start *State[T], fn func(s *State[T]) error) error {
	if start == nil {
		return errors.New("start state is nil")
	}
	visited := make(map[string]struct{})
	// Special case for first state.
	visited[start.label] = struct{}{} // Mark as visited.
	err := fn(start)
	if err != nil {
		return err
	}
	return walkStatesInternal(start, fn, visited)
}

func walkStatesInternal[T input](src *State[T], fn func(s *State[T]) error, visited map[string]struct{}) error {
	var toVisit []*State[T]
	for i := 0; i < len(src.transitions); i++ {
		if !statesEqual(src, src.transitions[i].Src) {
			panic("state's transition source \"" + src.String() + "\" not match transition source: " + src.transitions[i].String())
		}
		dst := src.transitions[i].Dst
		if _, ok := visited[dst.label]; ok {
			continue // Already visited.
		}
		visited[dst.label] = struct{}{} // Mark as visited.
		err := fn(dst)
		if err != nil {
			return err
		}
		toVisit = append(toVisit, dst)
	}
	for i := 0; i < len(toVisit); i++ {
		if err := walkStatesInternal(toVisit[i], fn, visited); err != nil {
			return err
		}
	}
	return nil
}

func walkTransitions[T input](start *State[T], fn func(s Transition[T]) error) error {
	if start == nil {
		return errors.New("start state is nil")
	}
	return WalkStates(start, func(s *State[T]) error {
		for i := 0; i < len(s.transitions); i++ {
			if err := fn(s.transitions[i]); err != nil {
				return err
			}
		}
		return nil
	})
}
