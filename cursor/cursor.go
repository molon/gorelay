package cursor

import "context"

type Counter[T any] interface {
	Count(ctx context.Context) (int, error)
}
