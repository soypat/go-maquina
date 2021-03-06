[![go.dev reference](https://pkg.go.dev/badge/github.com/soypat/go-maquina)](https://pkg.go.dev/github.com/soypat/go-maquina)
[![Go Report Card](https://goreportcard.com/badge/github.com/soypat/go-maquina)](https://goreportcard.com/report/github.com/soypat/go-maquina)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

# go-maquina

Create finite-state machines in Go. Inspired by [stateless](https://github.com/qmuntal/stateless).

_Maquina_ is the spanish word for machine. It is pronounced maa-kee-nuh, much like _machina_ from the Latin calque [_deus-ex-machina_](https://en.wikipedia.org/wiki/Deus_ex_machina).

## Code organization

* [`maquina.go`](./maquina.go) contains internal logic for the state machine such as the `fire()` functions triggered by a state transition.

* [`state.go`](./state.go) contains most of the user visible exported methods on `State` type.

* [`statemachine.go`](./statemachine.go) contains code relevant to the State manager StateMachine.


## Toll booth example

```go
	const (
		passageCost                      = 10.00
		defaultPay                       = 0.0
		payUp            maquina.Trigger = "customer pays"
		customerAdvances maquina.Trigger = "customer advances"
	)

	tollClosed := maquina.NewState("toll barrier closed", defaultPay)
	tollOpen := maquina.NewState("toll barrier open", defaultPay)

	tollClosed.Permit(payUp, tollOpen, func(_ context.Context, pay float64) error {
		if pay < passageCost {
			// Barrier remains closed unless customer pays up
			return fmt.Errorf("customer underpaid with $%.2f", pay)
		}
		return nil
	})
	tollOpen.Permit(customerAdvances, tollClosed)

	SM := maquina.NewStateMachine(tollClosed)
	for i := 0; i < 5; i++ {
		pay := 2 * passageCost * rand.Float64()
		// FireBg activates transition with context.Background()
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
