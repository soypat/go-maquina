package maquina

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"testing"
)

type intTransition = Transition[int]

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
	fcbP1 := NewFringeCallback("plus1", func(_ context.Context, _ intTransition, _ int) {
		i++
	})
	fcbP2 := NewFringeCallback("plus1", func(_ context.Context, _ intTransition, _ int) {
		i += 2
	})
	sm.OnTransitioning(fcbP1)
	sm.OnTransitioned(fcbP2)
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
	state1.OnReentry(NewFringeCallback("cb", func(ctx context.Context, _ intTransition, input int) {
		reentryCount++
	}))
	state1.Permit(trigger1_2, state2)
	state1.OnExit(NewFringeCallback("cb", func(ctx context.Context, _ intTransition, input int) {
		exitCount1++
	}))

	state2.Permit(trigger2_3, state3)
	state2.OnEntry(NewFringeCallback("cb", func(ctx context.Context, _ intTransition, input int) {
		entryCount2++
	}))
	state2.OnExit(NewFringeCallback("cb", func(ctx context.Context, _ intTransition, input int) {
		exitCount2++
	}))

	state3.Permit(trigger3_2, state2)
	state3.OnExit(NewFringeCallback("cb", func(ctx context.Context, _ intTransition, input int) {
		exitCount3++
	}))
	state3.OnEntry(NewFringeCallback("cb", func(ctx context.Context, _ intTransition, input int) {
		entryCount3++
	}))
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
	var countEntry1_2, countEntry2_3, countEntry3_2, countReentry1, countExit1_2 int
	state1.Permit(trigger1_2, state2)
	state1.Permit(triggerReentry1, state1)
	state1.OnReentryFrom(triggerReentry1, NewFringeCallback("cb", func(ctx context.Context, _ intTransition, input int) {
		countReentry1++
	}))
	state1.OnExitThrough(trigger1_2, NewFringeCallback("cb", func(ctx context.Context, _ intTransition, input int) {
		countExit1_2++
	}))

	state2.Permit(trigger2_3, state3)
	state2.OnEntryFrom(trigger1_2, NewFringeCallback("cb", func(ctx context.Context, _ intTransition, input int) {
		countEntry1_2++
	}))

	state2.OnEntryFrom(trigger3_2, NewFringeCallback("cb", func(ctx context.Context, _ intTransition, input int) {
		countEntry3_2++
	}))

	state3.Permit(trigger3_2, state2)
	state3.OnEntryFrom(trigger2_3, NewFringeCallback("cb", func(ctx context.Context, _ intTransition, input int) {
		countEntry2_3++
	}))

	sm := NewStateMachine(state1)
	err := sm.FireBg(triggerReentry1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if countReentry1 != 1 {
		t.Error("expected 1 reentry into state1, got", countReentry1)
	}
	if countExit1_2 != 0 {
		t.Error("expected 0 exit from state1, got", countExit1_2)
	}

	err = sm.FireBg(trigger1_2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if countExit1_2 != 1 {
		t.Error("expected 1 exit from state1 to state2, got", countExit1_2)
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

func hyperTrig(start, end int) Trigger {
	return Trigger("T" + strconv.Itoa(start) + "→" + strconv.Itoa(end))
}

func hyperStates(n int) []*State[int] {
	states := make([]*State[int], n)
	for i := 0; i < n; i++ {
		states[i] = NewState("S"+strconv.Itoa(i), i)
		for j := i - 1; j >= 0; j-- {
			trigger := hyperTrig(i, j)
			states[i].Permit(trigger, states[j])
		}
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			trigger := hyperTrig(i, j)
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
	var okState = NewState("ok1", 1)
	var nilGC func(_ context.Context, _ int) error
	var nilFringeCallback func(_ context.Context, _ intTransition, _ int)
	var nilState *State[int]
	var nilFringe FringeCallback[int]
	var okFringe = NewFringeCallback("cb", func(_ context.Context, _ intTransition, _ int) {})
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
			fn:   func() { NewState("ok", 1).OnExit(nilFringe) },
		},
		{
			desc: "nil on entry callback",
			fn:   func() { NewState("ok", 1).OnEntry(nilFringe) },
		},
		{
			desc: "nil on reentry callback",
			fn:   func() { NewState("ok", 1).OnReentry(nilFringe) },
		},
		{
			desc: "empty fringe label",
			fn:   func() { NewFringeCallback("", func(_ context.Context, _ intTransition, _ int) {}) },
		},
		{
			desc: "nil fringe callback",
			fn:   func() { NewFringeCallback("cb", nilFringeCallback) },
		},
		{
			desc: "use of trigger wildcard",
			fn:   func() { NewState("ok", 1).OnEntryFrom(triggerWildcard, okFringe) },
		},
		{
			desc: "empty trigger",
			fn:   func() { NewState("ok", 1).Permit("", NewState("notok", 2)) },
		},
		{
			desc: "trigger registered twice in state",
			fn: func() {
				s := NewState("ok", 1)
				s.Permit("trig1", s)
				s.Permit("trig1", s)
			},
		},
		{
			desc: "nil destination state in always permit",
			fn:   func() { NewStateMachine(okState).AlwaysPermit("ok", nil) },
		},
		{
			desc: "wildcard trigger in always permit",
			fn:   func() { NewStateMachine(okState).AlwaysPermit(triggerWildcard, okState) },
		},
		{
			desc: "empty trigger in always permit",
			fn:   func() { NewStateMachine(okState).AlwaysPermit("", okState) },
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("expected panic, got no panic")
				}
			}()
			tC.fn()
		})
	}
}

func ExampleWriteDOT_simple() {
	state1 := NewState("state1", 1)
	state2 := NewState("state2", 2)
	state1.Permit("trigger", state2)
	sm := NewStateMachine(state1)
	var buf bytes.Buffer
	_, err := WriteDOT(&buf, sm)
	if err != nil {
		panic(err)
	}
	fmt.Println(buf.String())
	//Unordered output:
	// digraph {
	//   rankdir=LR;
	//   node [shape = box];
	//   graph [ dpi = 300 ];
	//   "state1" -> "state2" [ label = "trigger", style = "solid" ];
	//   "state2" [ color = red ]
	//   "state1" [ color = blue ]
	// }
}

func TestTriggersAvailable(t *testing.T) {
	states := hyperStates(8)
	sm := NewStateMachine(states[0])
	tr := sm.TriggersAvailable()
	if len(tr) != 7 {
		t.Error("expected 7 triggers, got", len(tr))
	}
	for i := 0; i < len(tr); i++ {
		for j := i + 1; j < len(tr); j++ {
			if tr[i] == tr[j] {
				t.Errorf("expected unique triggers, got %d == %d", i, j)
			}
		}
	}
}

func TestLinkSubstates(t *testing.T) {
	states := hyperStates(5)
	parent := states[0]
	superParent := states[4]
	err := parent.LinkSubstates(states[1], states[2])
	if err != nil {
		t.Fatal(err)
	}
	err = superParent.LinkSubstates(parent)
	if err != nil {
		t.Fatal(err)
	}
	if !parent.isSubstateOf(superParent) {
		t.Error("expected parent to be substate of superParent")
	}

	if !states[1].isSubstateOf(parent) || !states[2].isSubstateOf(parent) {
		t.Error("expected linked substate")
	}
	if states[3].isSubstateOf(parent) {
		t.Error("did not expect linked substate")
	}

	if !states[1].isSubstateOf(superParent) || !states[2].isSubstateOf(superParent) {
		t.Error("expected linked substate")
	}
	if states[3].isSubstateOf(superParent) {
		t.Error("did not expect linked substate")
	}

	err = states[1].LinkSubstates(parent)
	if err == nil {
		t.Error("expected cyclic linking error, got nil")
	}
	err = parent.LinkSubstates(states[1])
	if err == nil {
		t.Error("expected error on linking already linked state, got nil")
	}
	err = parent.LinkSubstates(nil)
	if err == nil {
		t.Error("expected error on linking nil state, got nil")
	}
}

func TestSuperstateFringe(t *testing.T) {
	const (
		PARENT   = 0
		SUPER    = 4
		EXTERNAL = 3 // State with no parent.
	)
	states := hyperStates(5)
	parent := states[PARENT]
	superParent := states[SUPER]
	parent.LinkSubstates(states[1], states[2])
	superParent.LinkSubstates(parent)
	var enter, exit, superEnter, superExit int
	var lastTransiton intTransition
	var (
		fringeEnter = NewFringeCallback("enter", func(_ context.Context, _ intTransition, _ int) {
			enter++
		})
		fringeExit = NewFringeCallback("exit", func(_ context.Context, _ intTransition, _ int) {
			exit++
		})
		superFringeEnter = NewFringeCallback("superEnter", func(_ context.Context, _ intTransition, _ int) {
			superEnter++
		})
		superFringeExit = NewFringeCallback("superExit", func(_ context.Context, _ intTransition, _ int) {
			superExit++
		})
		onTransitioning = NewFringeCallback("onTransition", func(_ context.Context, tr intTransition, _ int) {
			lastTransiton = tr
		})
	)
	parent.OnEntry(fringeEnter)
	parent.OnExit(fringeExit)
	superParent.OnEntry(superFringeEnter)
	superParent.OnExit(superFringeExit)

	sm := NewStateMachine(states[1])
	sm.OnTransitioning(onTransitioning)

	expectEnter := 0
	expectExit := 0
	expectSEnter := 0
	expectSExit := 0
	for it, test := range []struct {
		START, END                int
		expectEnter, expectExit   int
		expectSEnter, expectSExit int
	}{
		0: {START: 1, END: PARENT},
		1: {START: PARENT, END: SUPER, expectExit: 1},
		2: {START: SUPER, END: EXTERNAL, expectSExit: 1},
		3: {START: EXTERNAL, END: 1, expectEnter: 1, expectSEnter: 1},
		4: {START: 1, END: EXTERNAL, expectExit: 1, expectSExit: 1},
		5: {START: EXTERNAL, END: SUPER, expectSEnter: 1},
		6: {START: SUPER, END: PARENT, expectEnter: 1},
		// Internal transitions.
		7: {START: PARENT, END: 1}, 8: {START: 1, END: 2}, 9: {START: 2, END: 1}, 10: {START: 1, END: 2},

		11: {START: 2, END: SUPER, expectExit: 1},
		12: {START: SUPER, END: 2, expectEnter: 1},
	} {
		trigger := hyperTrig(test.START, test.END)
		t.Run(fmt.Sprintf("iter=%d:%s", it, trigger), func(t *testing.T) {
			err := sm.FireBg(trigger, it)
			if err != nil {
				t.Fatal(err)
			}
			expectEnter += test.expectEnter
			expectExit += test.expectExit
			expectSEnter += test.expectSEnter
			expectSExit += test.expectSExit

			if enter != expectEnter {
				t.Errorf("unexpected ENTER %d!=%d", enter, expectEnter)
			}
			if exit != expectExit {
				t.Errorf("unexpected EXIT %d!=%d", exit, expectExit)
			}
			if superEnter != expectSEnter {
				t.Errorf("unexpected SUPENTER %d!=%d", superEnter, expectSEnter)
			}
			if superExit != expectSExit {
				t.Errorf("unexpected SUPEXIT %d!=%d", superExit, expectSExit)
			}
		})
		if t.Failed() {
			t.Errorf("lastTransition: %s", lastTransiton.String())
			t.FailNow()
		}
	}
}

func ExampleMermaid() {
	const (
		PARENT   = 0
		SUPER    = 4
		EXTERNAL = 3 // State with no parent.
	)
	states := hyperStates(5)
	parent := states[PARENT]
	superParent := states[SUPER]
	parent.LinkSubstates(states[1], states[2])
	superParent.LinkSubstates(parent)

	sm := NewStateMachine(parent)
	var buf bytes.Buffer
	writeMermaidStateDiagram(&buf, sm, diagConfig{})
	fmt.Println(buf.String())
	//Unordered output:

}

func BenchmarkHyper(b *testing.B) {
	rand.Seed(1)
	states := hyperStates(8)
	sm := NewStateMachine(states[0])
	sm.OnUnhandledTrigger(func(current *State[int], t Trigger) error {
		return nil
	})
	ctx := context.TODO()
	// avail := sm.TriggersPermitted(ctx, 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		avail := sm.actual.transitions // Avoid allocations.
		nextTrigger := avail[rand.Intn(len(avail))]
		err := sm.Fire(ctx, nextTrigger.Trigger, 1)
		if err != nil {
			b.Log("error", err)
		}
	}
}
