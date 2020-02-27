package lite

import (
	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/lite2/provider"
	"github.com/tendermint/tendermint/lite2/provider/http"
	"github.com/tendermint/tendermint/lite2/store"
	"time"
)

// NewClient returns a new light client. It returns an error if it fails to
// obtain the header & vals from the primary or they are invalid (e.g. trust
// hash does not match with the one from the header).
//
// Witnesses are providers, which will be used for cross-checking the primary
// provider. At least one witness must be given. A witness can become a primary
// iff the current primary is unavailable.
//
// See all Option(s) for the additional configuration.
func NewClient(
	chainID string,
	trustOptions TrustOptions,
	primary provider.Provider,
	witnesses []provider.Provider,
	trustedStore store.Store,
	options ...Option) (*Client, error) {

	if err := trustOptions.ValidateBasic(); err != nil {
		return nil, errors.Wrap(err, "invalid TrustOptions")
	}

	c, err := NewClientFromTrustedStore(chainID, trustOptions.Period, primary, witnesses, trustedStore, options...)
	if err != nil {
		return nil, err
	}

	if c.latestTrustedHeader != nil {
		if err := c.checkTrustedHeaderUsingOptions(trustOptions); err != nil {
			return nil, err
		}
	}

	if c.latestTrustedHeader == nil || c.latestTrustedHeader.Height < trustOptions.Height {
		if err := c.initializeWithTrustOptions(trustOptions); err != nil {
			return nil, err
		}
	}

	return c, err
}

// NewClientFromTrustedStore initializes existing client from the trusted store.
//
// See NewClient
func NewClientFromTrustedStore(
	chainID string,
	trustingPeriod time.Duration,
	primary provider.Provider,
	witnesses []provider.Provider,
	trustedStore store.Store,
	options ...Option) (*Client, error) {

	c := &Client{
		chainID:                            chainID,
		trustingPeriod:                     trustingPeriod,
		verificationMode:                   skipping,
		trustLevel:                         DefaultTrustLevel,
		maxRetryAttempts:                   defaultMaxRetryAttempts,
		primary:                            primary,
		witnesses:                          witnesses,
		trustedStore:                       trustedStore,
		updatePeriod:                       defaultUpdatePeriod,
		removeNoLongerTrustedHeadersPeriod: defaultRemoveNoLongerTrustedHeadersPeriod,
		confirmationFn:                     func(action string) bool { return true },
		quit:                               make(chan struct{}),
		logger:                             log.NewNopLogger(),
	}

	for _, o := range options {
		o(c)
	}

	// Validate the number of witnesses.
	if len(c.witnesses) < 1 {
		return nil, errors.New("expected at least one witness")
	}

	// Verify witnesses are all on the same chain.
	for i, w := range witnesses {
		if w.ChainID() != chainID {
			return nil, errors.Errorf("witness #%d: %v is on another chain %s, expected %s",
				i, w, w.ChainID(), chainID)
		}
	}

	// Validate trust level.
	if err := ValidateTrustLevel(c.trustLevel); err != nil {
		return nil, err
	}

	if err := c.restoreTrustedHeaderAndVals(); err != nil {
		return nil, err
	}

	return c, nil
}

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
	var providers []provider.Provider
	for _, address := range providerAddresses {
		singleProvider, err := http.New(chainID, address)
		if err != nil {
			return nil, err
		}
		providers = append(providers, singleProvider)
	}
	return providers, nil
}
