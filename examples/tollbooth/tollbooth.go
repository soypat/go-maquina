package main

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/soypat/go-maquina"
)

func main() {
	const (
		passageCost                      = 10.00
		defaultPay                       = 0.0
		payUp            maquina.Trigger = "customer pays"
		customerAdvances maquina.Trigger = "customer advances"
	)

	tollClosed := maquina.NewState("toll barrier closed", defaultPay)
	tollOpen := maquina.NewState("toll barrier open", defaultPay)

	tollClosed.Permit(payUp, tollOpen, func(_ context.Context, pay float64) bool {
		return pay >= passageCost // Barrier remains closed unless customer pays up
	})
	tollOpen.Permit(customerAdvances, tollClosed)

	SM := maquina.NewStateMachine(tollClosed)
	for i := 0; i < 10; i++ {
		pay := 2 * passageCost * rand.Float64()
		err := SM.FireBg(payUp, pay)
		if err != nil {
			fmt.Printf("customer payed:%.2f, not enough!\n", pay)
			continue
		}
		fmt.Printf("customer payed:%.2f, let them pass!\n", pay)
		SM.FireBg(customerAdvances, 0)
	}
}
