package prod

import (
	"context"

	"case-01/src/state"
)

func ProvideStore(ctx context.Context) (*state.Store, func(), error) {
	return &state.Store{Env: "prod"}, func() {}, nil
}
