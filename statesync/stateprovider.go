package statesync

import (
	"fmt"
	"strings"
	"sync"
	"time"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/libs/log"
	lite "github.com/tendermint/tendermint/lite2"
	liteprovider "github.com/tendermint/tendermint/lite2/provider"
	litehttp "github.com/tendermint/tendermint/lite2/provider/http"
	literpc "github.com/tendermint/tendermint/lite2/rpc"
	litedb "github.com/tendermint/tendermint/lite2/store/db"
	tmstate "github.com/tendermint/tendermint/proto/state"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

//go:generate mockery -case underscore -name StateProvider

// StateProvider is a provider of trusted state data for bootstrapping a node. This refers
// to the state.State object, not the state machine.
type StateProvider interface {
	// AppHash returns the app hash after the given height has been committed.
	AppHash(height uint64) ([]byte, error)
	// Commit returns the commit at the given height.
	Commit(height uint64) (*types.Commit, error)
	// State returns a state object at the given height.
	State(height uint64) (sm.State, error)
}

// lightClientStateProvider is a state provider using the light client.
type lightClientStateProvider struct {
	sync.Mutex // lite.Client is not concurrency-safe
	lc         *lite.Client
	version    tmstate.Version
	providers  map[liteprovider.Provider]string
}

// NewLightClientStateProvider creates a new StateProvider using a light client and RPC clients.
func NewLightClientStateProvider(
	chainID string,
	version tmstate.Version,
	servers []string,
	trustOptions lite.TrustOptions,
	logger log.Logger,
) (StateProvider, error) {
	if len(servers) < 2 {
		return nil, fmt.Errorf("at least 2 RPC servers are required, got %v", len(servers))
	}

	providers := make([]liteprovider.Provider, 0, len(servers))
	providerRemotes := make(map[liteprovider.Provider]string)
	for _, server := range servers {
		client, err := rpcClient(server)
		if err != nil {
			return nil, fmt.Errorf("failed to set up RPC client: %w", err)
		}
		provider := litehttp.NewWithClient(chainID, client)
		providers = append(providers, provider)
		// We store the RPC addresses keyed by provider, so we can find the address of the primary
		// provider used by the light client and use it to fetch consensus parameters.
		providerRemotes[provider] = server
	}

	lc, err := lite.NewClient(chainID, trustOptions, providers[0], providers[1:],
		litedb.New(dbm.NewMemDB(), ""), lite.Logger(logger), lite.MaxRetryAttempts(5))
	if err != nil {
		return nil, err
	}
	return &lightClientStateProvider{
		lc:        lc,
		version:   version,
		providers: providerRemotes,
	}, nil
}

// AppHash implements StateProvider.
func (s *lightClientStateProvider) AppHash(height uint64) ([]byte, error) {
	s.Lock()
	defer s.Unlock()

	// We have to fetch the next height, which contains the app hash for the previous height.
	header, err := s.lc.VerifyHeaderAtHeight(int64(height+1), time.Now())
	if err != nil {
		return nil, err
	}
	return header.AppHash, nil
}

// Commit implements StateProvider.
func (s *lightClientStateProvider) Commit(height uint64) (*types.Commit, error) {
	s.Lock()
	defer s.Unlock()
	header, err := s.lc.VerifyHeaderAtHeight(int64(height), time.Now())
	if err != nil {
		return nil, err
	}
	return header.Commit, nil
}

// State implements StateProvider.
func (s *lightClientStateProvider) State(height uint64) (sm.State, error) {
	s.Lock()
	defer s.Unlock()

	state := sm.State{
		ChainID: s.lc.ChainID(),
		Version: s.version,
	}

	// We need to verify up until h+2, to get the validator set. This also prefetches the headers
	// for h and h+1 in the typical case where the trusted header is after the snapshot height.
	_, err := s.lc.VerifyHeaderAtHeight(int64(height+2), time.Now())
	if err != nil {
		return sm.State{}, err
	}
	header, err := s.lc.VerifyHeaderAtHeight(int64(height), time.Now())
	if err != nil {
		return sm.State{}, err
	}
	nextHeader, err := s.lc.VerifyHeaderAtHeight(int64(height+1), time.Now())
	if err != nil {
		return sm.State{}, err
	}
	state.LastBlockHeight = header.Height
	state.LastBlockTime = header.Time
	state.LastBlockID = header.Commit.BlockID
	state.AppHash = nextHeader.AppHash
	state.LastResultsHash = nextHeader.LastResultsHash

	state.LastValidators, _, err = s.lc.TrustedValidatorSet(int64(height))
	if err != nil {
		return sm.State{}, err
	}
	state.Validators, _, err = s.lc.TrustedValidatorSet(int64(height + 1))
	if err != nil {
		return sm.State{}, err
	}
	state.NextValidators, _, err = s.lc.TrustedValidatorSet(int64(height + 2))
	if err != nil {
		return sm.State{}, err
	}
	state.LastHeightValidatorsChanged = int64(height)

	// We'll also need to fetch consensus params via RPC, using light client verification.
	primaryURL, ok := s.providers[s.lc.Primary()]
	if !ok || primaryURL == "" {
		return sm.State{}, fmt.Errorf("could not find address for primary light client provider")
	}
	primaryRPC, err := rpcClient(primaryURL)
	if err != nil {
		return sm.State{}, fmt.Errorf("unable to create RPC client: %w", err)
	}
	rpcclient := literpc.NewClient(primaryRPC, s.lc)
	result, err := rpcclient.ConsensusParams(&nextHeader.Height)
	if err != nil {
		return sm.State{}, fmt.Errorf("unable to fetch consensus parameters for height %v: %w",
			nextHeader.Height, err)
	}
	state.ConsensusParams = result.ConsensusParams

	return state, nil
}

// rpcClient sets up a new RPC client
func rpcClient(server string) (*rpchttp.HTTP, error) {
	if !strings.Contains(server, "://") {
		server = "http://" + server
	}
	c, err := rpchttp.New(server, "/websocket")
	if err != nil {
		return nil, err
	}
	return c, nil
}
