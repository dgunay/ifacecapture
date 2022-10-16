package util

// Any returns true if the given slice contains an element satisfying the predicate.
func Any[T any](elems []T, pred func(T) bool) bool {
	for _, elem := range elems {
		if pred(elem) {
			return true
		}
	}

	return false
}
