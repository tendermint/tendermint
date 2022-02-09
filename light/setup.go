package light

import (
	"context"

	"github.com/tendermint/tendermint/light/provider"
	"github.com/tendermint/tendermint/light/provider/http"
	"github.com/tendermint/tendermint/light/store"
)

// NewHTTPClient initiates an instance of a light client using HTTP addresses
// for both the primary provider and witnesses of the light client. A trusted
// header and hash must be passed to initialize the client.
//
// See all Option(s) for the additional configuration.
// See NewClient.
func NewHTTPClient(
	ctx context.Context,
	chainID string,
	trustOptions TrustOptions,
	primaryAddress string,
	witnessesAddresses []string,
	trustedStore store.Store,
	options ...Option) (*Client, error) {

	providers, err := providersFromAddresses(append(witnessesAddresses, primaryAddress), chainID)
	if err != nil {
		return nil, err
	}

	return NewClient(
		ctx,
		chainID,
		trustOptions,
		providers[len(providers)-1],
		providers[:len(providers)-1],
		trustedStore,
		options...)
}

func providersFromAddresses(addrs []string, chainID string) ([]provider.Provider, error) {
	providers := make([]provider.Provider, len(addrs))
	for idx, address := range addrs {
		p, err := http.New(chainID, address)
		if err != nil {
			return nil, err
		}
		providers[idx] = p
	}
	return providers, nil
}
