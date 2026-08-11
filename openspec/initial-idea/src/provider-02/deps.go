package provider02

import (
	"case-01/src/c"
	"case-01/src/d"
	"case-01/src/state"
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

// ProvideStore is the shared fallback for any State output variant with no
// local override of its own — demonstrating a variant resolving Store()
// from the global scope. Prod overrides Store() with its own local provider
// in src/state/prod; Staging has no local provider directory of its own
// (created on demand, empty) and falls back to this one.
func ProvideStore(ctx context.Context) (*state.Store, func(), error) {
	return &state.Store{Env: "shared"}, func() {}, nil
}
