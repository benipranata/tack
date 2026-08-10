package d

import "context"

func NewD(ctx context.Context) D {
	return D{}
}

type D struct {
	foo string
}
