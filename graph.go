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
	ngot, err := w.Write([]byte("digraph {\n  rankdir=LR;\n  node [shape = box];\n  graph [ dpi = 300 ];\n"))
	n += ngot
	if err != nil {
		return n, err
	}
	isSource := true
	err = WalkStates(sm.actual, func(s *State[T]) error {
		if s.isSink() {
			ngot, err = fmt.Fprintf(w, "  %q [ color = red ]\n", s.label)
			n += ngot
			if err != nil {
				return err
			}
		}

		for i := 0; i < len(s.transitions); i++ {
			tr := s.transitions[i]
			ngot, err = writeDOTentry(w, tr)
			n += ngot
			if err != nil {
				return err
			}
			if isSource && statesEqual(sm.actual, tr.Dst) {
				isSource = false
			}
		}
		return nil
	})
	if err != nil {
		return n, err
	}
	if isSource {
		ngot, err = fmt.Fprintf(w, "  %q [ color = blue ]\n", sm.actual.label)
		n += ngot
	}
	if err != nil {
		return n, err
	}
	ngot, err = w.Write([]byte("}\n"))
	n += ngot
	return n, err
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
