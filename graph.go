package maquina

import (
	"fmt"
	"io"
)

// WriteDOT writes the DOT representation of the state machine to w,
// DOT being the graph description language used by Graphviz.
// It uses ForEachTransition to traverse the state machine starting
// at the start state.
func WriteDOT[T input](w io.Writer, sm *StateMachine[T]) (n int, err error) {
	// always := make(map[string]*Transition[T])
	// for i := 0; i < len(start.alwaysPermitted); i++ {
	// 	always[start.alwaysPermitted[i].Trigger.String()] = &start.alwaysPermitted[i]
	// }
	ngot, err := w.Write([]byte("digraph {\n  rankdir=LR;\n  node [shape = box];\n  graph [ dpi = 300 ];\n"))
	n += ngot
	if err != nil {
		return n, err
	}

	err = walkTransitions(sm.actual, func(tr Transition[T]) error {
		ngot, err = writeDOTentry(w, tr)
		n += ngot
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return n, err
	}
	ngot, err = w.Write([]byte("}\n"))
	n += ngot
	return n, err
	// for s := range states {
	// 	for i := range sm.alwaysPermitted {
	// 		if s.hasTransition(sm.alwaysPermitted[i].Trigger) {
	// 			break
	// 		}
	// 		if i == len(sm.alwaysPermitted)-1 {
	// 			// Copy transition to avoid modifying the original with source state.
	// 			transition := sm.alwaysPermitted[i]
	// 			transition.Src = s
	// 			// Write always permitted transitions if not existing in state.
	// 			ngot, err = writeDOTentry(w, transition)
	// 			n += ngot
	// 			if err != nil {
	// 				return n, err
	// 			}
	// 		}
	// 	}
	// }

}

func writeDOTentry[T input](w io.Writer, tr Transition[T]) (int, error) {
	var style string = "solid"
	if tr.HasGuards() {
		style = "dashed"
	}
	label := tr.Trigger.String()
	for i := range tr.guards {
		label += "\n[" + tr.guards[i].label + "]"
	}
	return fmt.Fprintf(w, "  %q -> %q [ label = %q, style = %q ];\n", tr.Src.label, tr.Dst.label, label, style)
}
