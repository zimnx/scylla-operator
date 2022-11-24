package helpers

func ToArray[T any](objs ...T) []T {
	res := make([]T, 0, len(objs))
	return append(res, objs...)
}
