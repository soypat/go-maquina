[![go.dev reference](https://pkg.go.dev/badge/github.com/soypat/go-maquina)](https://pkg.go.dev/github.com/soypat/go-maquina)
[![Go Report Card](https://goreportcard.com/badge/github.com/soypat/go-maquina)](https://goreportcard.com/report/github.com/soypat/go-maquina)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![codecov](https://codecov.io/gh/soypat/go-maquina/branch/main/graph/badge.svg?token=5DH2RG1UVP)](https://codecov.io/gh/soypat/go-maquina)

# go-maquina

Create delightfully simple finite-state machines in Go. Inspired by [stateless](https://github.com/qmuntal/stateless).

If you wish to build state machines that are extremely maintainable and stand the test of time
you have come to the right place. 

## Highlights
- Labelled everything: Label your states, triggers, guard clauses and callbacks so that...
	- You can visualize state machines as a DOT generated graph. See examples below!
	- Deep introspection into what is going on in your state machine.
- Decent performance, no allocations: see benchmark below.


_Maquina_ is the spanish word for machine. It is pronounced maa-kee-nuh, much like _machina_ from the Latin calque [_deus-ex-machina_](https://en.wikipedia.org/wiki/Deus_ex_machina).

### Benchmark
Benchmarked below is the time it takes for a transition to complete when no callbacks or guard clauses are in place.
```
$ go test -test.bench=. -benchmem
goos: linux
goarch: amd64
pkg: github.com/soypat/go-maquina
cpu: 12th Gen Intel(R) Core(TM) i5-12400F
BenchmarkHyper-12       31407175                37.38 ns/op            0 B/op        0 allocs/op
PASS
ok      github.com/soypat/go-maquina    2.192s
```

## Code organization

* [`maquina.go`](./maquina.go) contains internal logic for the state machine such as the `fire()` functions triggered by a state transition.

* [`state.go`](./state.go) contains most of the user visible exported methods on `State` type.

* [`statemachine.go`](./statemachine.go) contains code relevant to the State manager StateMachine.


## Toll booth example
![toolbooth diagram](https://user-images.githubusercontent.com/26156425/238150418-c223b843-ae14-4694-a40c-c6b123c43886.png)
```go
const (
	passageCost                      = 10.00
	defaultPay                       = 0.0
	payUp            maquina.Trigger = "customer pays"
	customerAdvances maquina.Trigger = "customer advances"
)
var (
	tollClosed = maquina.NewState("toll barrier closed", defaultPay)
	tollOpen   = maquina.NewState("toll barrier open", defaultPay)
	guardPay   = maquina.NewGuard("payment check", func(ctx context.Context, pay float64) error {
		if pay < passageCost {
			// Barrier remains closed unless customer pays up
			return fmt.Errorf("customer underpaid with $%.2f", pay)
		}
		return nil
	})
)

tollClosed.Permit(payUp, tollOpen, guardPay)
tollOpen.Permit(customerAdvances, tollClosed)
SM := maquina.NewStateMachine(tollClosed)
for i := 0; i < 5; i++ {
	pay := 2 * passageCost * rand.Float64()
	err := SM.FireBg(payUp, pay)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("customer paid $%.2f, let them pass!\n", pay)
		SM.FireBg(customerAdvances, 0)
	}
}
```
The code above outputs:

```
customer paid $12.09, let them pass!
customer paid $18.81, let them pass!
customer paid $13.29, let them pass!
guard clause failed: customer underpaid with $8.75
guard clause failed: customer underpaid with $8.49
```
## Algorithmic trading graph
The code below outputs the following DOT graph code. Note how parent/super states can be crafted. Entry/Exit callbacks will be triggered on a superstate when entering/exiting a substate from outside/within the super state.

![algorithmic trading example](https://user-images.githubusercontent.com/26156425/253705810-b716999f-1863-48e7-af86-3d3119f75100.png)


```go
getStock := func() string {
		return string([]byte{byte(rand.Intn(26)) + 'A', byte(rand.Intn(26)) + 'A', byte(rand.Intn(26)) + 'A'})
	}
	type tradeState struct {
		targetStock   string
		quoteReceived time.Time
	}
	type transition = maquina.Transition[*tradeState]

	const (
		trigRequestQuote     = "request quote"
		trigExecute          = "execute"
		trigExecuteFail      = "execute failed"
		trigCancel           = "cancel"
		trigQuoteReceived    = "quote received"
		trigExecuteConfirmed = "execute confirmed"
	)
	var (
		stateWaitingOnQuote = maquina.NewState("waiting on quote", &tradeState{})
		stateIdle           = maquina.NewState("idle", &tradeState{})
		stateExecuting      = maquina.NewState("executing", &tradeState{})
		stateCritical       = maquina.NewState("critical", &tradeState{})

		fringeStockSelect = maquina.NewFringeCallback("stock select", func(_ context.Context, _ transition, state *tradeState) {
			state.targetStock = getStock()
		})

		fringeStockClear = maquina.NewFringeCallback("stock clear", func(_ context.Context, _ transition, state *tradeState) {
			state.targetStock = ""
		})

		guardQuoteStale = maquina.NewGuard("quote stale", func(ctx context.Context, state *tradeState) error {
			const staleQuoteTimeout = 10 * time.Minute
			elapsed := time.Since(state.quoteReceived)
			if elapsed > staleQuoteTimeout || elapsed < 1 { // Sanity check included.
				return errors.New("quote is stale: " + elapsed.String() + " elapsed")
			}
			return nil
		})
	)

	stateIdle.Permit(trigRequestQuote, stateWaitingOnQuote)
	stateIdle.OnExitThrough(trigRequestQuote, fringeStockSelect)
	stateIdle.OnEntry(fringeStockClear)

	stateWaitingOnQuote.Permit(trigExecute, stateExecuting, guardQuoteStale)
	stateWaitingOnQuote.Permit(trigCancel, stateIdle)

	stateExecuting.Permit(trigExecuteConfirmed, stateIdle)
	stateExecuting.Permit(trigExecuteFail, stateWaitingOnQuote)

	// Mark critical section as a superstate.
	stateCritical.LinkSubstates(stateWaitingOnQuote, stateExecuting)

	sm := maquina.NewStateMachine(stateIdle)
	var buf bytes.Buffer
	maquina.WriteDOT2(&buf, sm)
	fmt.Println(buf.String())
```

## 3D Printer graphviz example
The code below outputs the following DOT graph code:
![3d printer example](https://user-images.githubusercontent.com/26156425/238145938-6cf54057-ae07-4b47-ad54-d3997032d540.png)

```go
type printerState struct {
	x, y, z int
}
// Declaration of triggers. These are actions.
// In the example of a 3D printer one could think of them
// as buttons exposed to the end user.
const (
	trigHome      maquina.Trigger = "home"
	trigCalibrate maquina.Trigger = "calibrate"
	trigStop      maquina.Trigger = "stop"
)
var (
	// stateSingleton contains the state of the printer at all times.
	// It is a singleton and is shared by all states.
	stateSingleton   = &printerState{}
	stateIdleHome    = maquina.NewState("idle at home", stateSingleton)
	stateIdle        = maquina.NewState("idle", stateSingleton)
	stateCalibrating = maquina.NewState("calibrating", stateSingleton)
	stateGoingHome   = maquina.NewState("going home", stateSingleton)
	// guardNotAtHome is a guard clause that checks if the printer is at home position.
	guardNotAtHome = maquina.NewGuard("not at home", func(ctx context.Context, state *printerState) error {
		if state.x != 0 || state.y != 0 || state.z != 0 {
			return fmt.Errorf("not at home")
		}
		return nil
	})
)
// Declare Calibration and Stop transitions. These would be the actions taken
// when user presses CALIBRATE or STOP button.
stateIdleHome.Permit(trigCalibrate, stateCalibrating)
stateIdle.Permit(trigCalibrate, stateCalibrating, guardNotAtHome)
// Special case of STOP while home: we stay at home.
stateIdleHome.Permit(trigStop, stateIdleHome)

// Declare home transitions. These would be the actions taken when a user presses
// the HOME button, as an example.
stateCalibrating.Permit(trigHome, stateGoingHome)
stateIdle.Permit(trigHome, stateGoingHome)
stateGoingHome.Permit(trigHome, stateIdleHome, guardNotAtHome)
sm := maquina.NewStateMachine(stateIdleHome)
// In the case of stopping we go to Idle state since we are not
// guaranteed to be at home position.
sm.AlwaysPermit(trigStop, stateIdle)
var buf bytes.Buffer
maquina.WriteDOT(&buf, sm)
fmt.Println(buf.String())
// With the code below one can also output a PNG file with the graph:
// One must have graphviz installed and in the path: `sudo apt install graphviz`
//
//  cmd := exec.Command("dot", "-Tpng", "-o ", "3dprinter.png")
//  cmd.Stdin = &buf
//  cmd.Run()
```

## Hyper connected state diagram
A toy example of 8 states, all of them connected to illustrate capabilities of go-maquina when coupled to graphviz (code is below):
![hyper-states](https://user-images.githubusercontent.com/26156425/238158584-b178ecce-ea0c-4a8b-987b-5e4cc7986ad8.png)

```go
const n = 8
hyperStates := make([]maquina.State[int], n)
for i := 0; i < n; i++ {
	hyperStates[i] = *maquina.NewState("S"+strconv.Itoa(i), i)
	for j := i - 1; j >= 0; j-- {
		trigger := maquina.Trigger("T" + strconv.Itoa(i) + "→" + strconv.Itoa(j))
		hyperStates[i].Permit(trigger, &hyperStates[j])
	}
}
for i := 0; i < n; i++ {
	for j := i + 1; j < n; j++ {
		trigger := maquina.Trigger("T" + strconv.Itoa(i) + "→" + strconv.Itoa(j))
		hyperStates[i].Permit(trigger, &hyperStates[j])
	}
}
sourceState := maquina.NewState("source", 0)
sourceState.Permit("goto S0", &hyperStates[0])
failsafeState := maquina.NewState("sink failsafe", -1)
sm := maquina.NewStateMachine(sourceState)
sm.AlwaysPermit("goto failsafe", failsafeState)
var buf bytes.Buffer
maquina.WriteDOT(&buf, sm)
cmd := exec.Command("dot", "-Tpng", "-o", "hyper-states.png")
cmd.Stdin = &buf
cmd.Run()
```
