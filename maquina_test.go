package maquina

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"testing"
)

func TestStateMachine_OnUnhandledTrigger(t *testing.T) {
	var unhandled = errors.New("unhandled")
	sm := NewStateMachine(NewState("start", 1))
	sm.OnUnhandledTrigger(func(current *State[int], t Trigger) error {
		return unhandled
	})
	err := sm.FireBg("unhandled trigger", 1)
	if err != unhandled {
		t.Errorf("expected unhandled error, got %v", err)
	}
	sm.OnUnhandledTrigger(nil)

	t.Run("catch panic", func(t *testing.T) {
		defer func() {
			a := recover()
			if a == nil {
				t.Error("expected panic with unhandled trigger, got nil")
			}
		}()
		sm.FireBg("unhandled trigger with panic", 1)
	})
}

func TestStateMachine_alwaysPermit(t *testing.T) {
	const alwaysTrig Trigger = "goto failsafe"
	failsafeState := NewState("failsafe", 1)
	states := hyperStates(8)
	sm := NewStateMachine(states[0])
	sm.AlwaysPermit(alwaysTrig, failsafeState)
	// makeDOT("failsafe_alwayspermit", sm)
	i := 0
	sm.OnTransitioning(func(s Transition[int]) {
		i += 1
	})
	sm.OnTransitioned(func(s Transition[int]) {
		i += 2
	})
	err := sm.FireBg(alwaysTrig, 1)
	if err != nil {
		t.Error("expected no error, got", err)
	}
	if sm.State() != failsafeState {
		t.Errorf("expected %s, got %s", failsafeState, sm.State())
	}
	expect := 3
	if i != expect {
		t.Errorf("expected both transitioning callbacks to be called, got counter %d instead of %d", i, expect)
	}
	err = sm.FireBg(alwaysTrig, 1)
	if err != nil {
		t.Error("expected no error, got", err)
	}
	if sm.State() != failsafeState {
		t.Errorf("expected %s, got %s", failsafeState.String(), sm.State())
	}
	expect = 6
	if i != expect {
		t.Errorf("expected both transitioning callbacks to be called, got counter %d instead of %d", i, expect)
	}

}

func TestWalkStates(t *testing.T) {
	const hyperNum = 8
	states := hyperStates(hyperNum)
	statesCounted := 0
	WalkStates(states[0], func(s *State[int]) error {
		statesCounted++
		return nil
	})
	if statesCounted != hyperNum {
		t.Error("expected", hyperNum, "states, got", statesCounted)
	}
	// sm := NewStateMachine(states[0])
	// makeDOT("hyper", sm)
}

func TestGuardClauseError(t *testing.T) {
	var guardError = errors.New("guard error")
	state1 := NewState("state1", 1)
	state2 := NewState("state2", 2)
	state1.Permit("trigger", state2, NewGuard("always fail", func(_ context.Context, _ int) error {
		return guardError
	}))
	sm := NewStateMachine(state1)
	err := sm.FireBg("trigger", 1)
	if !errors.Is(err, guardError) {
		t.Errorf("expected guard error, got %v", err)
	}
	var g *GuardClauseError
	if !errors.As(err, &g) {
		t.Errorf("expected guard clause error, got %T", err)
	}
}

func hyperStates(n int) []*State[int] {
	states := make([]*State[int], n)
	for i := 0; i < n; i++ {
		states[i] = NewState("S"+strconv.Itoa(i), i)
		for j := i - 1; j >= 0; j-- {
			trigger := Trigger("T" + strconv.Itoa(i) + "→" + strconv.Itoa(j))
			states[i].Permit(trigger, states[j])
		}
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			trigger := Trigger("T" + strconv.Itoa(i) + "→" + strconv.Itoa(j))
			states[i].Permit(trigger, states[j])
		}
	}
	return states
}

func makeDOT(name string, sm *StateMachine[int]) {
	var buf bytes.Buffer
	_, err := WriteDOT(&buf, sm)
	if err != nil {
		panic(err)
	}
	os.Mkdir("testdata", 0777)
	cmd := exec.Command("dot", "-Tpng", "-o", "testdata/"+name+".png")
	cmd.Stdin = &buf
	cmd.Run()
}
