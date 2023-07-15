package maquina

import (
	"fmt"
	"io"
)

// WriteDOT writes the DOT representation of the state machine to w,
// DOT being the graph description language used by Graphviz.
// See http://www.graphviz.org/ for more information.
//
// A few things to note about the output:
//   - States are shown as boxes with their label as name.
//   - Transitions are shown as arrows from the source state to the destination state
//     labelled with the trigger.
//   - Transitions with guards are shown as dashed arrows and their guards are
//     listed below the transition trigger label surrounded by square brackets.
//   - States with only exiting transitions are shown in blue ("sources" in graph theory).
//     Due to internal state representation only the state machine's start state can be a source.
//     These states once left cannot be re-entered.
//   - States with only entering transitions are shown in red ("sinks" in graph theory).
//     These states once reached cannot be exited.
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

func WriteDOT2[T input](w io.Writer, sm *StateMachine[T]) (n int, err error) {
	ngot, err := w.Write([]byte("digraph {\n  rankdir=LR;\n  node [shape = box];\n  graph [ dpi = 300 ];\n"))
	n += ngot
	if err != nil {
		return n, err
	}
	isSource := true
	superStates := make(map[string][]*State[T])
	err = WalkStates(sm.actual, func(s *State[T]) error {
		if s.isSink() {
			ngot, err = fmt.Fprintf(w, "  %q [ color = red ]\n", s.label)
			n += ngot
			if err != nil {
				return err
			}
		}
		if s.parent != nil {
			superStates[s.parent.label] = append(superStates[s.parent.label], s)
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
	if err == nil && isSource {
		ngot, err = fmt.Fprintf(w, "  %q [ color = blue ]\n", sm.actual.label)
		n += ngot
	}
	if err != nil {
		return n, err
	}
	i := 0
	for label, substates := range superStates {
		ngot, err = fmt.Fprintf(w, "  subgraph cluster_%x {\n    label = %q;\n", i, label)
		n += ngot
		i++
		if err != nil {
			return n, err
		}
		for _, s := range substates {
			ngot, err = fmt.Fprintf(w, "    %q;\n", s.label)
			n += ngot
			if err != nil {
				return n, err
			}
		}
		ngot, err = fmt.Fprintf(w, "  }\n")
		n += ngot
		if err != nil {
			return n, err
		}
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
