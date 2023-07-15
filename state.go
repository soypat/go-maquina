package maquina

import (
	"errors"
)

// State basic functional unit of a finite state machine.
// The T type parameter is the type of the input argument received by
// entry, exit, reentry and guard clause callbacks during state transitions.
// Two states are equal to eachother if they have identical labels.
type State[T input] struct {
	label        string
	transitions  []Transition[T]
	exitFuncs    []triggeredFunc[T]
	entryFuncs   []triggeredFunc[T]
	reentryFuncs []triggeredFunc[T]
	parent       *State[T]
}

// NewState instantiates a state with a label for tracking and tracing.
// The type parameter T will be the argument received by entry, exit,
// reentry and guard clause callbacks during state transitions.
func NewState[T input](label string, _ T) *State[T] {
	// input T // TODO(soypat): Should this be implemented someday? Ideas:
	// - Add a method called FireDefault to statemachine that uses this input
	// - Use this for fuzzing a StateMachine as a starting seed input?
	if label == "" {
		panic("label cannot be empty")
	}
	return &State[T]{
		label: label,
	}
}

// Label returns the label with which the state was created. Does not heap allocate.
func (s *State[T]) Label() string { return s.label }

// LinkSubstates links argument states as substates of the receiver state s.
func (s *State[T]) LinkSubstates(substates ...*State[T]) error {
	for i := range substates {
		if substates[i] == nil {
			return errors.New("cannot link nil state")
		}
		if substates[i].parent != nil {
			return errors.New("state " + substates[i].Label() + " already has parent " + substates[i].parent.Label())
		}
		if s.IsSubstateOf(substates[i]) {
			return errors.New("making " + substates[i].Label() + " a substate of " + s.Label() + " would cause a referential cycle")
		}
		substates[i].parent = s
	}

	return nil
}

// IsSubstateOf returns true if the receiver state s is a substate of the given
// maybeParent state or if states are equal to each other.
func (s *State[T]) IsSubstateOf(maybeParent *State[T]) bool {
	if maybeParent == nil {
		return false
	}
	for s != nil {
		if statesEqual(s, maybeParent) {
			return true
		}
		s = s.parent
	}
	return false
}

// String returns a pretty-printed representation of the state and its transitions
// separated by newlines.
func (s State[T]) String() (str string) {
	str += s.label + ":\n"
	for i := 0; i < len(s.transitions); i++ {
		str += "\t" + s.transitions[i].String() + "\n"
	}
	return str
}

// Permit registers a state transition from receiver s to dst when Trigger t is
// invoked given the guard clauses return true. If any of the guard clauses return
// false the state transition is aborted and the Fire() attempt by the state machine
// returns an error.
func (s *State[T]) Permit(t Trigger, dst *State[T], guards ...GuardClause[T]) {
	if dst == nil {
		panic("nil destination state")
	}
	s.validateForPermit(t)
	s.transitions = append(s.transitions, Transition[T]{
		Src: s, Dst: dst, Trigger: t, guards: guards,
	})
}

// OnEntryFrom registers a callback that executes on entering State s
// through Trigger t. Does not execute on reentry.
func (s *State[T]) OnEntryFrom(t Trigger, fcb FringeCallback[T]) {
	t.mustNotBeWildcard()
	s.onEntryInternal(t, fcb)
}

// OnEntry registers a callback that executes on entering State s.
// Does not execute on reentry.
func (s *State[T]) OnEntry(fcb FringeCallback[T]) {
	s.onEntryInternal(triggerWildcard, fcb)
}

// OnExitThrough registers a callback that executes on exiting State s
// through Trigger t. Does not execute on reentry.
func (s *State[T]) OnExitThrough(t Trigger, fcb FringeCallback[T]) {
	t.mustNotBeWildcard()
	s.onExitInternal(t, fcb)
}

// OnExit registers a callback that executes on exiting State s.
// Does not execute on reentry.
func (s *State[T]) OnExit(fcb FringeCallback[T]) {
	s.onExitInternal(triggerWildcard, fcb)
}

// OnReentry registers a callback that executes when reentering State s.
func (s *State[T]) OnReentry(fcb FringeCallback[T]) {
	s.onReentryInternal(triggerWildcard, fcb)
}

// OnReentryFrom registers a callback that executes when reentering State s through Trigger t.
func (s *State[T]) OnReentryFrom(t Trigger, fcb FringeCallback[T]) {
	t.mustNotBeWildcard()
	s.onReentryInternal(t, fcb)
}

// OnExitCallbacks returns all callbacks registered to execute on exiting State s
// when trigger t is fired. If t is the empty string all callbacks registered to
// execute on exiting State s are returned.
func (s *State[T]) OnExitCallbacks(t Trigger) (callbacks []FringeCallback[T]) {
	return findCallbacks(t, s.exitFuncs)
}

// OnEntryCallbacks returns all callbacks registered to execute on entering State s
// when trigger t is fired. If t is the empty string all callbacks registered to
// execute on entering State s are returned.
func (s *State[T]) OnEntryCallbacks(t Trigger) (callbacks []FringeCallback[T]) {
	return findCallbacks(t, s.entryFuncs)
}

// OnReentryCallbacks returns all callbacks registered to execute on reentering
// State s when trigger t is fired. If t is the empty string all callbacks
// registered to execute on reentering State s are returned.
func (s *State[T]) OnReentryCallbacks(t Trigger) (callbacks []FringeCallback[T]) {
	return findCallbacks(t, s.reentryFuncs)
}

func findCallbacks[T input](t Trigger, fns []triggeredFunc[T]) (callbacks []FringeCallback[T]) {
	if t == triggerWildcard {
		return nil
	}
	if t == "" {
		t = triggerWildcard
	}
	for i := 0; i < len(fns); i++ {
		if triggersEqual(t, fns[i].t) {
			callbacks = append(callbacks, fns[i].f)
		}
	}
	return callbacks
}
func (s *State[T]) hasTransition(t Trigger) bool {
	for i := 0; i < len(s.transitions); i++ {
		if s.transitions[i].Trigger == t {
			return true
		}
	}
	return false
}

// isSink returns true if the state has no outgoing transitions.
func (s *State[T]) isSink() bool {
	for i := 0; i < len(s.transitions); i++ {
		if !statesEqual(s, s.transitions[i].Dst) {
			return false
		}
	}
	return true
}

func (s *State[T]) onExitInternal(t Trigger, fcb FringeCallback[T]) {
	if fcb.cb == nil {
		panic("onExit function cannot be nil")
	}
	s.exitFuncs = append(s.exitFuncs, triggeredFunc[T]{
		f: fcb,
		t: t,
	})
}

func (s *State[T]) onReentryInternal(t Trigger, fcb FringeCallback[T]) {
	if fcb.cb == nil {
		panic("onReentry function cannot be nil")
	}
	s.reentryFuncs = append(s.reentryFuncs, triggeredFunc[T]{
		f: fcb,
		t: t,
	})
}

func (s *State[T]) onEntryInternal(t Trigger, f FringeCallback[T]) {
	if f.cb == nil {
		panic("onEntry function cannot be nil")
	}
	s.entryFuncs = append(s.entryFuncs, triggeredFunc[T]{
		f: f,
		t: t,
	})
}

var errTriggerWildcardNotAllowed = errors.New("trigger " + triggerWildcard.Quote() + " reserved for internal use (wildcard)")

func (s *State[T]) validateForPermit(t Trigger) {
	t.mustNotBeWildcard()
	existingTransition := s.getTransition(t)
	if existingTransition != nil {
		panic("trigger " + t.Quote() + " already registered as transition: " + existingTransition.String())
	}
}

func (t Trigger) mustNotBeWildcard() {
	switch t {
	case "":
		// we also perform an empty trigger check since that is never allowed.
		panic("trigger must not be empty string")
	case triggerWildcard:
		panic(errTriggerWildcardNotAllowed)
	}
}
