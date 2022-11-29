package infra

import (
	"context"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// Provider defines an API for manipulating the infrastructure of a
// specific set of testnet infrastructure.
type Provider interface {

	// Setup generates any necessary configuration for the infrastructure
	// provider during testnet setup.
	Setup() error

	StartTendermint(context.Context, *e2e.Node) error
	KillTendermint(context.Context, *e2e.Node) error
	TerminateTendermint(context.Context, *e2e.Node) error
}

// NoopProvider implements the provider interface by performing noops for every
// interface method. This may be useful if the infrastructure is managed by a
// separate process.
type NoopProvider struct {
}

func (NoopProvider) Setup() error                                             { return nil }
func (NoopProvider) StartTendermint(_ context.Context, _ *e2e.Node) error     { return nil }
func (NoopProvider) TerminateTendermint(_ context.Context, _ *e2e.Node) error { return nil }
func (NoopProvider) KillTendermint(_ context.Context, _ *e2e.Node) error      { return nil }

var _ Provider = NoopProvider{}
