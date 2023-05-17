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

func TestOnReentryExitEntry(t *testing.T) {
	const (
		triggerReentry1 Trigger = "reentry1"
		trigger1_2      Trigger = "t1-2"
		trigger2_3      Trigger = "t2-3"
		trigger3_2      Trigger = "t3-2"
	)
	state1 := NewState("state1", 1)
	state2 := NewState("state2", 2)
	state3 := NewState("state3", 3)
	var reentryCount, exitCount1, entryCount2, exitCount2, entryCount3, exitCount3 int

	state1.Permit(triggerReentry1, state1)
	state1.OnReentry(func(ctx context.Context, input int) {
		reentryCount++
	})
	state1.Permit(trigger1_2, state2)
	state1.OnExit(func(ctx context.Context, input int) {
		exitCount1++
	})

	state2.Permit(trigger2_3, state3)
	state2.OnEntry(func(ctx context.Context, input int) {
		entryCount2++
	})
	state2.OnExit(func(ctx context.Context, input int) {
		exitCount2++
	})

	state3.Permit(trigger3_2, state2)
	state3.OnExit(func(ctx context.Context, input int) {
		exitCount3++
	})
	state3.OnEntry(func(ctx context.Context, input int) {
		entryCount3++
	})
	sm := NewStateMachine(state1)
	// Perform reentries.
	const reeentries = 5
	for i := 0; i < reeentries; i++ {
		err := sm.FireBg(triggerReentry1, 1)
		if err != nil {
			t.Fatal(err)
		}
	}
	if reentryCount != reeentries {
		t.Errorf("expected %d reentries, got %d", reeentries, reentryCount)
	}
	err := sm.FireBg(trigger1_2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if exitCount1 != 1 {
		t.Error("expected 1 exit from state1, got", exitCount1)
	}
	if entryCount2 != 1 {
		t.Error("expected 1 entry into state2, got", entryCount2)
	}
	const transitions = 5
	for i := 0; i < transitions; i++ {
		err := sm.FireBg(trigger2_3, 1)
		if err != nil {
			t.Fatal(err)
		}
		err = sm.FireBg(trigger3_2, 1)
		if err != nil {
			t.Fatal(err)
		}
		if exitCount2 != i+1 {
			t.Errorf("expected %d exits from state2, got %d", i+1, exitCount2)
		}
		if entryCount3 != i+1 {
			t.Errorf("expected %d exits from state2, got %d", i+1, exitCount2)
		}

		if exitCount3 != i+1 {
			t.Errorf("expected %d exits from state3, got %d", i+1, exitCount3)
		}
		if entryCount2 != i+2 {
			t.Errorf("expected %d exits from state2, got %d", i+2, exitCount2)
		}
	}
}

func TestOnEntryExitFrom(t *testing.T) {
	const (
		triggerReentry1 Trigger = "reentry1"
		trigger1_2      Trigger = "t1-2"
		trigger2_3      Trigger = "t2-3"
		trigger3_2      Trigger = "t3-2"
	)
	state1 := NewState("state1", 1)
	state2 := NewState("state2", 2)
	state3 := NewState("state3", 3)
	var countEntry1_2, countEntry2_3, countEntry3_2, countReentry1 int
	state1.Permit(trigger1_2, state2)
	state1.Permit(triggerReentry1, state1)
	state1.OnReentryFrom(triggerReentry1, func(ctx context.Context, input int) {
		countReentry1++
	})

	state2.Permit(trigger2_3, state3)
	state2.OnEntryFrom(trigger1_2, func(ctx context.Context, input int) {
		countEntry1_2++
	})

	state2.OnEntryFrom(trigger3_2, func(ctx context.Context, input int) {
		countEntry3_2++
	})

	state3.Permit(trigger3_2, state2)
	state3.OnEntryFrom(trigger2_3, func(ctx context.Context, input int) {
		countEntry2_3++
	})

	sm := NewStateMachine(state1)
	err := sm.FireBg(triggerReentry1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if countReentry1 != 1 {
		t.Error("expected 1 reentry into state1, got", countReentry1)
	}
	err = sm.FireBg(trigger1_2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if countEntry1_2 != 1 {
		t.Error("expected 1 entry into state2 from state1, got", countEntry1_2)
	}
	if countEntry3_2 != 0 {
		t.Error("expected 0 entry into state2 from state3, got", countEntry3_2)
	}

	const transitions = 5
	for i := 0; i < transitions; i++ {
		err = sm.FireBg(trigger2_3, 1)
		if err != nil {
			t.Fatal(err)
		}
		if countEntry2_3 != i+1 {
			t.Errorf("expected %d entries into state3 from state2, got %d", i+1, countEntry2_3)
		}
		if countEntry3_2 != i {
			t.Errorf("expected %d entries into state2 from state3, got %d", i, countEntry3_2)
		}
		err = sm.FireBg(trigger3_2, 1)
		if err != nil {
			t.Fatal(err)
		}
		if countEntry2_3 != i+1 {
			t.Errorf("expected %d entries into state3 from state2, got %d", i+1, countEntry2_3)
		}
		if countEntry3_2 != i+1 {
			t.Errorf("expected %d entries into state2 from state3, got %d", i+1, countEntry3_2)
		}
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

func TestMustPanics(t *testing.T) {
	var nilGC func(_ context.Context, _ int) error
	var nilState *State[int]
	testCases := []struct {
		desc string
		fn   func()
	}{
		{
			desc: "empty state label",
			fn:   func() { NewState("", 1) },
		},
		{
			desc: "empty guard clause label",
			fn:   func() { NewGuard("", func(_ context.Context, _ int) error { return nil }) },
		},
		{
			desc: "nil guard clause callback",
			fn:   func() { NewGuard("ok", nilGC) },
		},
		{
			desc: "nil destination state",
			fn:   func() { NewState("ok", 1).Permit("ok", nil) },
		},
		{
			desc: "nil state machine state",
			fn:   func() { NewStateMachine(nilState) },
		},
		{
			desc: "nil on exit callback",
			fn:   func() { NewState("ok", 1).OnExit(nil) },
		},
		{
			desc: "nil on entry callback",
			fn:   func() { NewState("ok", 1).OnEntry(nil) },
		},
		{
			desc: "nil on reentry callback",
			fn:   func() { NewState("ok", 1).OnReentry(nil) },
		},
		{
			desc: "use of trigger wildcard",
			fn:   func() { NewState("ok", 1).OnEntryFrom(triggerWildcard, func(ctx context.Context, input int) {}) },
		},
		{
			desc: "empty trigger",
			fn:   func() { NewState("ok", 1).Permit("", NewState("notok", 2)) },
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("expected panic, got nil")
				}
			}()
			tC.fn()
		})
	}
}
