package maquina_test

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/soypat/go-maquina"
)

func ExampleStateMachine_tollBooth() {
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
		err := SM.FireBg(payUp, pay)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("customer paid $%.2f, let them pass!\n", pay)
			SM.FireBg(customerAdvances, 0)
		}
	}
	//Output:
	// customer paid $12.09, let them pass!
	// customer paid $18.81, let them pass!
	// customer paid $13.29, let them pass!
	// guard clause failed: customer underpaid with $8.75
	// guard clause failed: customer underpaid with $8.49
}
