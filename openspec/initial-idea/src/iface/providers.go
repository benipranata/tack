package iface

import (
	"case-01/src/c"
	"context"
)

func ProvideC(ctx context.Context) (*c.C, func(), error) {
	cClient := &c.C{Foo: "specific"}
	return cClient, func() { cClient.Close() }, nil
}
