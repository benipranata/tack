package provider02

import (
	"case-01/src/c"
	"case-01/src/d"
	"context"
)

func ProvideC(ctx context.Context) (*c.C, func(), error) {
	cClient := &c.C{Foo: "bar"}
	return cClient, func() { cClient.Close() }, nil
}

func ProvideYYY(ctx context.Context) (*d.D, func(), error) {
	dClient := d.NewD(ctx)
	return &dClient, func() {}, nil
}
