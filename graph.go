package maquina

func ForEachTransition[T input](start *State[T], fn func(tr Transition[T]) error) error {
	visited := make(map[string]struct{})
	return forEachTransition(start, fn, visited)
}

func forEachTransition[T input](src *State[T], fn func(tr Transition[T]) error, visited map[string]struct{}) error {
	if _, ok := visited[src.label]; ok {
		return nil
	}
	visited[src.label] = struct{}{}
	for i := 0; i < len(src.transitions); i++ {
		if !statesEqual(src, src.transitions[i].Src) {
			panic("state's transition source not match self: " + src.String() + " != " + src.transitions[i].Src.String())
		}
		err := fn(src.transitions[i])
		if err != nil {
			return err
		}
		err = forEachTransition(src.transitions[i].Dst, fn, visited)
		if err != nil {
			return err
		}
	}
	return nil
}
