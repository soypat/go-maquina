package maquina_test

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"

	"github.com/soypat/go-maquina"
)

func ExampleStateMachine_tollBooth() {
	rand.Seed(1)
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
	//Output:
	// customer paid $12.09, let them pass!
	// customer paid $18.81, let them pass!
	// customer paid $13.29, let them pass!
	// guard clause "payment check" failed: customer underpaid with $8.75
	// guard clause "payment check" failed: customer underpaid with $8.49
}

func ExampleToDOT_3dPrinter() {
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
	//  cmd := exec.Command("dot", "-Tpng", "-o", "3dprinterNoBug.png")
	//  cmd.Stdin = &buf
	//  cmd.Run()

	// Unordered output:
	// digraph {
	//   rankdir=LR;
	//   node [shape = box];
	//   graph [ dpi = 300 ];
	//   "idle at home" -> "calibrating" [ label = "calibrate", style = "solid" ];
	//   "calibrating" -> "going home" [ label = "home", style = "solid" ];
	//   "going home" -> "idle at home" [ label = "home\n[not at home]", style = "dashed" ];
	//   "calibrating" -> "idle" [ label = "stop", style = "solid" ];
	//   "idle" -> "calibrating" [ label = "calibrate\n[not at home]", style = "dashed" ];
	//   "idle" -> "going home" [ label = "home", style = "solid" ];
	//   "idle at home" -> "idle at home" [ label = "stop", style = "solid" ];
	//   "going home" -> "idle" [ label = "stop", style = "solid" ];
	//   "idle" -> "idle" [ label = "stop", style = "solid" ];
	// }
}
