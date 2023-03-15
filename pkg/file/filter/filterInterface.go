package filter

type HaveFilterInterface[T any] interface {
	HaveFilter(msg T, keyWord []T) *string
}
