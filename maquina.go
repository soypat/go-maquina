package maquina

import (
	"context"
	"errors"
)

type input = any

type State[T input] struct {
	defaultInput T
	label        string
	transitions  []Transition[T]
	exitFuncs    []triggeredFunc[T]
	entryFuncs   []triggeredFunc[T]
	reentryFuncs []triggeredFunc[T]
}

type triggeredFunc[T input] struct {
	t Trigger
	f FringeFunc[T]
}

type GuardClause[T input] func(ctx context.Context, input T) bool

type FringeFunc[T input] func(ctx context.Context, input T)

type Transition[T input] struct {
	Src     *State[T]
	Dst     *State[T]
	Trigger Trigger
	guards  []GuardClause[T]
}

type Trigger string

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
	panic("could not find firing trigger " + string(t) + " registered with state " + s.label)
}

func (s *State[T]) getTransition(t Trigger) *Transition[T] {
	for i := range s.transitions {
		if triggersEqual(t, s.transitions[i].Trigger) {
			return &s.transitions[i]
		}
	}
	return nil
}

func (s Transition[T]) fire(ctx context.Context, t Trigger, input T) error {
	for i := range s.guards {
		if !s.guards[i](ctx, input) {
			return errors.New("guard clause failed")
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if statesEqual(s.Src, s.Dst) {
		s.Src.reenter(ctx, t, input)
		return nil
	}
	s.Src.exit(ctx, t, input)
	s.Dst.enter(ctx, t, input)
	return nil
}
