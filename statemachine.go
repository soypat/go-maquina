package maquina

import "context"

type StateMachine[T input] struct {
	fireOp uint64
	actual *State[T]
}

func NewStateMachine[T input](s *State[T]) *StateMachine[T] {
	return &StateMachine[T]{
		actual: s,
	}
}

func (sm *StateMachine[T]) FireBg(t Trigger, input T) error {
	return sm.Fire(context.Background(), t, input)
}

func (sm *StateMachine[T]) Fire(ctx context.Context, t Trigger, input T) error {
	transition := sm.actual.getTransition(t)
	if transition == nil {
		panic("undefined trigger handling not supported yet")
	}
	err := sm.actual.fire(ctx, t, input)
	if err != nil {
		return err
	}
	sm.actual = transition.Dst
	return nil
}
