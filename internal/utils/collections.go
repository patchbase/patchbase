package utils

// Map applies the function fn to each element of the input slice in and returns a new slice containing the results.
func Map[T any, R any](in []T, fn func(T) R) []R {
	out := make([]R, len(in))
	for i, v := range in {
		out[i] = fn(v)
	}
	return out
}

// MapErr applies the function fn to each element of the input slice in and returns a new slice containing the results.
// If fn returns an error for any element, MapErr returns that error immediately.
func MapErr[T any, R any](in []T, fn func(T) (R, error)) ([]R, error) {
	out := make([]R, len(in))
	for i, v := range in {
		r, err := fn(v)
		if err != nil {
			return nil, err
		}
		out[i] = r
	}
	return out, nil
}
