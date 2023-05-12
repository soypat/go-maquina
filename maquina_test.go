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
	rand.Seed(1)
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

func ExampleForEachTransition_3dPrinter() {
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
	)
	// Declare Calibration and Stop transitions. These would be the actions taken
	// when user presses CALIBRATE or STOP button.
	stateIdleHome.Permit(trigCalibrate, stateCalibrating)
	stateIdleHome.Permit(trigStop, stateIdleHome) // Reentry when stopping when already home.
	stateIdle.Permit(trigCalibrate, stateCalibrating)
	stateIdle.Permit(trigStop, stateIdle) // Reentry when stopping when already home.
	// In the case of stopping a calibration procedure we go to Idle state since we are not
	// guaranteed to be at home position.
	stateCalibrating.Permit(trigStop, stateIdle)
	// Declare home transitions. These would be the actions taken when a user presses
	// the HOME button, as an example.
	stateCalibrating.Permit(trigHome, stateGoingHome)
	stateIdle.Permit(trigHome, stateGoingHome)
	maquina.ForEachTransition(stateIdleHome, func(tr maquina.Transition[*printerState]) error {
		fmt.Println(tr.String(), fmt.Sprintf(" (guards:%v)", tr.HasGuards()))
		return nil
	})
	//Output:
	// idle at home --calibrate-> calibrating  (guards:false)
	// calibrating --stop-> idle  (guards:false)
	// idle --calibrate-> calibrating  (guards:false)
	// idle --stop-> idle  (guards:false)
	// idle --home-> going home  (guards:false)
	// calibrating --home-> going home  (guards:false)
	// idle at home --stop-> idle at home  (guards:false)
}
