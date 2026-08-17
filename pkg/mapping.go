package pkg

func MapAll[S any, T any](values []S, mf func(S) T) []T {
	res := make([]T, len(values))
	for i := range values {
		res[i] = mf(values[i])
	}
	return res
}

func MapAllErr[S any, T any](values []S, mf func(S) (T, error)) ([]T, error) {
	res := make([]T, len(values))
	for i := range values {
		mapped, err := mf(values[i])
		if err != nil {
			return nil, err
		}
		res[i] = mapped
	}
	return res, nil
}
