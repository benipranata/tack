package provider01

import (
	"case-01/src/a"
	"case-01/src/b"
	"context"
)

func ProvideA(ctx context.Context) (a.IA, func(), error) {
	iaClient := &a.A{X: "dummy"}
	return iaClient, func() { _ = iaClient.Close() }, nil
}

func ProvideXXX(ctx context.Context) (b.IB, func(), error) {
	ibClient, err := b.NewB()
	if err != nil {
		return nil, func() {}, err
	}
	return ibClient, func() {}, nil
}
