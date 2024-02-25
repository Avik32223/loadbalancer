package slices

func Filter[T any](ss []T, predicate func(T) bool) (ret []T) {
	for _, s := range ss {
		if predicate(s) {
			ret = append(ret, s)
		}
	}
	return
}
