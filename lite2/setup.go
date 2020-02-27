package lite

import (
	"time"

	"github.com/tendermint/tendermint/lite2/provider"
	"github.com/tendermint/tendermint/lite2/provider/http"
	"github.com/tendermint/tendermint/lite2/store"
)

// NewNetClient and NewNetClientFromTrustedStore each initiate an instance of a lite client using just RPC addresses
// to form the providers to the lite client
func NewNetClient(
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
		chainID,
		trustOptions,
		providers[len(providers)-1],
		providers[:len(providers)-1],
		trustedStore,
		options...)

}

func NewNetClientFromTrustedStore(
	chainID string,
	trustingPeriod time.Duration,
	primaryAddress string,
	witnessesAddresses []string,
	trustedStore store.Store,
	options ...Option) (*Client, error) {

	providers, err := providersFromAddresses(append(witnessesAddresses, primaryAddress), chainID)
	if err != nil {
		return nil, err
	}

	return NewClientFromTrustedStore(
		chainID,
		trustingPeriod,
		providers[len(providers)-1],
		providers[:len(providers)-1],
		trustedStore,
		options...)

}

func providersFromAddresses(providerAddresses []string, chainID string) ([]provider.Provider, error) {
	providers := make([]provider.Provider, len(providerAddresses))
	for idx, address := range providerAddresses {
		singleProvider, err := http.New(chainID, address)
		if err != nil {
			return nil, err
		}
		providers[idx] = singleProvider
	}
	return providers, nil
}
