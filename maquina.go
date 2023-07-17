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
// transition to complete succesfully. If a GuardClause returns an error
// during a transition the transition halts and the state remains as before.
type GuardClause[T input] struct {
	label string
	guard func(ctx context.Context, input T) error
}

// String returns the label with which gc was created.
func (gc GuardClause[T]) String() string { return gc.label }

// NewGuard instantiates a new GuardClause with a label and a guard function.
func NewGuard[T input](label string, guard func(ctx context.Context, input T) error) GuardClause[T] {
	if guard == nil {
		panic("nil guard clause callback")
	} else if label == "" {
		panic("empty guard clause label")
	}
	return GuardClause[T]{label: label, guard: guard}
}

type triggeredFunc[T input] struct {
	t Trigger
	f FringeCallback[T]
}

// FringeCallback represents a callback that executes on the fringe of a state
// transition. It can be used to execute code on entry, exit or reentry of a state.
type FringeCallback[T input] struct {
	label string
	cb    func(ctx context.Context, tr Transition[T], input T)
}

// String returns the label with which cb was created.
func (cb FringeCallback[T]) String() string { return cb.label }

// NewFringeCallback instantiates a new FringeCallback with a label and a callback
// that is executed on the fringe of a state transition.
// The label is used for printing of the callback and does not need to be unique.
func NewFringeCallback[T input](label string, callback func(ctx context.Context, tr Transition[T], input T)) FringeCallback[T] {
	if label == "" {
		panic("empty fringe callback label")
	} else if callback == nil {
		panic("nil fringe callback function")
	}
	return FringeCallback[T]{label: label, cb: callback}
}

// String returns the trigger string with which it was created.
func (t Trigger) String() string { return string(t) }

// Quote returns the trigger string with which it was create enclosed in double quotes.
func (t Trigger) Quote() string { return "\"" + t.String() + "\"" }

// triggerWildcard is a special trigger that matches any trigger during
// comparisons to check if a callback should be run. These include exit, reentry
// and entry callbacks. Wildcards are set internally by go-maquina for callbacks
// which should always run regardless of the transition.
const triggerWildcard Trigger = "*"

// triggersEqual checks if a trigger is equal to another trigger or the wildcard.
// Should only be used for checking if a callback should be run.
func triggersEqual(a, b Trigger) bool          { return a == b || a == triggerWildcard || b == triggerWildcard }
func statesEqual[T input](a, b *State[T]) bool { return a.label == b.label }

func (sm *StateMachine[T]) exit(ctx context.Context, tr Transition[T], input T) {
	if tr.Dst.parent != nil && tr.Src.Contains(tr.Dst) {
		return // Do not exit parent state if transitioning to a substate.
	}
	s := tr.Src
	for i := 0; i < len(s.exitFuncs); i++ {
		if triggersEqual(s.exitFuncs[i].t, tr.Trigger) {
			fringe := s.exitFuncs[i].f
			if sm.onFringe != nil {
				sm.onFringe(tr, fringe, input)
			}
			fringe.cb(ctx, tr, input)
		}
	}
	if tr.Src.parent != nil && !tr.Src.parent.Contains(tr.Dst) {
		newTr := tr
		newTr.Src = tr.Src.parent
		sm.exit(ctx, newTr, input)
	}
}

func (sm *StateMachine[T]) enter(ctx context.Context, tr Transition[T], input T) {
	if tr.Src.parent != nil && tr.Dst.Contains(tr.Src) {
		return // Transition from a substate.
	}
	if tr.Dst.parent != nil && !tr.Dst.parent.Contains(tr.Src) {
		newTr := tr
		newTr.Dst = tr.Dst.parent
		sm.enter(ctx, newTr, input)
	}
	s := tr.Dst
	for i := 0; i < len(s.entryFuncs); i++ {
		if triggersEqual(s.entryFuncs[i].t, tr.Trigger) {
			fringe := s.entryFuncs[i].f
			if sm.onFringe != nil {
				sm.onFringe(tr, fringe, input)
			}
			fringe.cb(ctx, tr, input)
		}
	}
}

func (sm *StateMachine[T]) reenter(ctx context.Context, tr Transition[T], input T) {
	s := tr.Dst
	for i := 0; i < len(s.reentryFuncs); i++ {
		if triggersEqual(s.reentryFuncs[i].t, tr.Trigger) {
			fringe := s.reentryFuncs[i].f
			if sm.onFringe != nil {
				sm.onFringe(tr, fringe, input)
			}
			fringe.cb(ctx, tr, input)
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
func (sm *StateMachine[T]) fire(ctx context.Context, tr Transition[T], input T) error {
	if err := tr.isPermitted(ctx, input); err != nil {
		return err
	}
	if statesEqual(tr.Src, tr.Dst) {
		sm.reenter(ctx, tr, input)
		return nil
	}
	sm.exit(ctx, tr, input)
	sm.enter(ctx, tr, input)
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

// GuardClauseError is a auxiliary type used to wrap errors returned by guard clauses
// so that users may check for them specifically after a call to Fire methods on
// a state machine:
//
//	err := sm.FireBg(trigger, input)
//	var g *GuardClauseError
//	if errors.As(err, &g) {
//		fmt.Println("guard label:", g.Label, "failed with error:", g.Unwrap())
//	}
//
// GuardClauseError also implements the Unwrap method so that users may access the
// original error returned by the guard clause or check for specific errors returned:
//
//	err := sm.FireBg(trigger, input)
//	if errors.Is(err, ErrFoo) {
//	  // handle ErrFoo
//	}
type GuardClauseError struct {
	// The guard clause label.
	Label string
	// The error as returned by the guard clause.
	err error
}

// Error returns a string representation of the error encountered by a guard clause
// and the guard clause label.
func (g GuardClauseError) Error() string {
	return "guard clause \"" + g.Label + "\" failed: " + g.err.Error()
}

// Unwrap returns the error encountered by a guard as returned by the GuardClause.
func (g GuardClauseError) Unwrap() error { return g.err }

func (tr Transition[T]) isPermitted(ctx context.Context, input T) error {
	for i := 0; i < len(tr.guards); i++ {
		if err := tr.guards[i].guard(ctx, input); err != nil {
			return &GuardClauseError{err: err, Label: tr.guards[i].label}
		}
		ctxErr := ctx.Err()
		if ctxErr != nil {
			return ctxErr
		}
	}
	return nil
}

// String returns a basic text-arrow representation of the transition.
func (tr Transition[T]) String() string {
	str := tr.Src.label + " --" + tr.Trigger.String() + "-> " + tr.Dst.label
	for i := 0; i < len(tr.guards); i++ {
		str += " [" + tr.guards[i].String() + "]"
	}
	return str
}

// WalkStates recurses down the state tree in a depth first search for
// all unique states in what would be a state machine starting with the argument state.
// It calls fn on every new state it finds. If fn returns an error, the walk is aborted
// and the error is returned.
//
// Beware that this function provides a view into the state machine's transitions
// and modifying them willy-nilly can cause undefined behavior.
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
