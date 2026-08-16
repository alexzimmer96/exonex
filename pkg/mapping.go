package pkg

func MapAll[S any, T any](values []S, mf func(S) T) []T {
	res := make([]T, len(values))
	for i := range values {
		res[i] = mf(values[i])
	}
	return res
}
