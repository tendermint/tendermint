package statesync

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/internal/p2p"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/light"
	lightprovider "github.com/tendermint/tendermint/light/provider"
	lighthttp "github.com/tendermint/tendermint/light/provider/http"
	lightrpc "github.com/tendermint/tendermint/light/rpc"
	lightdb "github.com/tendermint/tendermint/light/store/db"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

//go:generate ../../scripts/mockery_generate.sh StateProvider

// StateProvider is a provider of trusted state data for bootstrapping a node. This refers
// to the state.State object, not the state machine. There are two implementations. One
// uses the P2P layer and the other uses the RPC layer. Both use light client verification.
type StateProvider interface {
	// AppHash returns the app hash after the given height has been committed.
	AppHash(ctx context.Context, height uint64) ([]byte, error)
	// Commit returns the commit at the given height.
	Commit(ctx context.Context, height uint64) (*types.Commit, error)
	// State returns a state object at the given height.
	State(ctx context.Context, height uint64) (sm.State, error)
}

type stateProviderRPC struct {
	sync.Mutex    // light.Client is not concurrency-safe
	lc            *light.Client
	initialHeight int64
	providers     map[lightprovider.Provider]string
	logger        log.Logger
}

// NewRPCStateProvider creates a new StateProvider using a light client and RPC clients.
func NewRPCStateProvider(
	ctx context.Context,
	chainID string,
	initialHeight int64,
	servers []string,
	trustOptions light.TrustOptions,
	logger log.Logger,
) (StateProvider, error) {
	if len(servers) < 2 {
		return nil, fmt.Errorf("at least 2 RPC servers are required, got %d", len(servers))
	}

	providers := make([]lightprovider.Provider, 0, len(servers))
	providerRemotes := make(map[lightprovider.Provider]string)
	for _, server := range servers {
		client, err := rpcClient(server)
		if err != nil {
			return nil, fmt.Errorf("failed to set up RPC client: %w", err)
		}
		provider := lighthttp.NewWithClient(chainID, client)
		providers = append(providers, provider)
		// We store the RPC addresses keyed by provider, so we can find the address of the primary
		// provider used by the light client and use it to fetch consensus parameters.
		providerRemotes[provider] = server
	}

	lc, err := light.NewClient(ctx, chainID, trustOptions, providers[0], providers[1:],
		lightdb.New(dbm.NewMemDB()), light.Logger(logger))
	if err != nil {
		return nil, err
	}
	return &stateProviderRPC{
		logger:        logger,
		lc:            lc,
		initialHeight: initialHeight,
		providers:     providerRemotes,
	}, nil
}

func (s *stateProviderRPC) verifyLightBlockAtHeight(ctx context.Context, height uint64, ts time.Time) (*types.LightBlock, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return s.lc.VerifyLightBlockAtHeight(ctx, int64(height), ts)
}

// AppHash implements part of StateProvider. It calls the application to verify the
// light blocks at heights h+1 and h+2 and, if verification succeeds, reports the app
// hash for the block at height h+1 which correlates to the state at height h.
func (s *stateProviderRPC) AppHash(ctx context.Context, height uint64) ([]byte, error) {
	s.Lock()
	defer s.Unlock()

	// We have to fetch the next height, which contains the app hash for the previous height.
	header, err := s.verifyLightBlockAtHeight(ctx, height+1, time.Now())
	if err != nil {
		return nil, err
	}

	// We also try to fetch the blocks at H+2, since we need these
	// when building the state while restoring the snapshot. This avoids the race
	// condition where we try to restore a snapshot before H+2 exists.
	_, err = s.verifyLightBlockAtHeight(ctx, height+2, time.Now())
	if err != nil {
		return nil, err
	}
	return header.AppHash, nil
}

// Commit implements StateProvider.
func (s *stateProviderRPC) Commit(ctx context.Context, height uint64) (*types.Commit, error) {
	s.Lock()
	defer s.Unlock()
	header, err := s.verifyLightBlockAtHeight(ctx, height, time.Now())
	if err != nil {
		return nil, err
	}
	return header.Commit, nil
}

// State implements StateProvider.
func (s *stateProviderRPC) State(ctx context.Context, height uint64) (sm.State, error) {
	s.Lock()
	defer s.Unlock()

	state := sm.State{
		ChainID:       s.lc.ChainID(),
		InitialHeight: s.initialHeight,
	}
	if state.InitialHeight == 0 {
		state.InitialHeight = 1
	}

	// The snapshot height maps onto the state heights as follows:
	//
	// height: last block, i.e. the snapshotted height
	// height+1: current block, i.e. the first block we'll process after the snapshot
	// height+2: next block, i.e. the second block after the snapshot
	//
	// We need to fetch the NextValidators from height+2 because if the application changed
	// the validator set at the snapshot height then this only takes effect at height+2.
	lastLightBlock, err := s.verifyLightBlockAtHeight(ctx, height, time.Now())
	if err != nil {
		return sm.State{}, err
	}
	currentLightBlock, err := s.verifyLightBlockAtHeight(ctx, height+1, time.Now())
	if err != nil {
		return sm.State{}, err
	}
	nextLightBlock, err := s.verifyLightBlockAtHeight(ctx, height+2, time.Now())
	if err != nil {
		return sm.State{}, err
	}

	state.Version = sm.Version{
		Consensus: currentLightBlock.Version,
		Software:  version.TMVersion,
	}
	state.LastBlockHeight = lastLightBlock.Height
	state.LastBlockTime = lastLightBlock.Time
	state.LastBlockID = lastLightBlock.Commit.BlockID
	state.AppHash = currentLightBlock.AppHash
	state.LastResultsHash = currentLightBlock.LastResultsHash
	state.LastValidators = lastLightBlock.ValidatorSet
	state.Validators = currentLightBlock.ValidatorSet
	state.NextValidators = nextLightBlock.ValidatorSet
	state.LastHeightValidatorsChanged = nextLightBlock.Height

	// We'll also need to fetch consensus params via RPC, using light client verification.
	primaryURL, ok := s.providers[s.lc.Primary()]
	if !ok || primaryURL == "" {
		return sm.State{}, fmt.Errorf("could not find address for primary light client provider")
	}
	primaryRPC, err := rpcClient(primaryURL)
	if err != nil {
		return sm.State{}, fmt.Errorf("unable to create RPC client: %w", err)
	}
	rpcclient := lightrpc.NewClient(s.logger, primaryRPC, s.lc)
	result, err := rpcclient.ConsensusParams(ctx, &currentLightBlock.Height)
	if err != nil {
		return sm.State{}, fmt.Errorf("unable to fetch consensus parameters for height %v: %w",
			nextLightBlock.Height, err)
	}
	state.ConsensusParams = result.ConsensusParams
	state.LastHeightConsensusParamsChanged = currentLightBlock.Height

	return state, nil
}

// rpcClient sets up a new RPC client
func rpcClient(server string) (*rpchttp.HTTP, error) {
	if !strings.Contains(server, "://") {
		server = "http://" + server
	}
	return rpchttp.New(server)
}

type stateProviderP2P struct {
	sync.Mutex    // light.Client is not concurrency-safe
	lc            *light.Client
	initialHeight int64
	paramsSendCh  p2p.Channel
	paramsRecvCh  chan types.ConsensusParams
}

// NewP2PStateProvider creates a light client state
// provider but uses a dispatcher connected to the P2P layer
func NewP2PStateProvider(
	ctx context.Context,
	chainID string,
	initialHeight int64,
	providers []lightprovider.Provider,
	trustOptions light.TrustOptions,
	paramsSendCh p2p.Channel,
	logger log.Logger,
) (StateProvider, error) {
	if len(providers) < 2 {
		return nil, fmt.Errorf("at least 2 peers are required, got %d", len(providers))
	}

	lc, err := light.NewClient(ctx, chainID, trustOptions, providers[0], providers[1:],
		lightdb.New(dbm.NewMemDB()), light.Logger(logger))
	if err != nil {
		return nil, err
	}

	return &stateProviderP2P{
		lc:            lc,
		initialHeight: initialHeight,
		paramsSendCh:  paramsSendCh,
		paramsRecvCh:  make(chan types.ConsensusParams),
	}, nil
}

func (s *stateProviderP2P) verifyLightBlockAtHeight(ctx context.Context, height uint64, ts time.Time) (*types.LightBlock, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return s.lc.VerifyLightBlockAtHeight(ctx, int64(height), ts)
}

// AppHash implements StateProvider.
func (s *stateProviderP2P) AppHash(ctx context.Context, height uint64) ([]byte, error) {
	s.Lock()
	defer s.Unlock()

	// We have to fetch the next height, which contains the app hash for the previous height.
	header, err := s.verifyLightBlockAtHeight(ctx, height+1, time.Now())
	if err != nil {
		return nil, err
	}

	// We also try to fetch the blocks at H+2, since we need these
	// when building the state while restoring the snapshot. This avoids the race
	// condition where we try to restore a snapshot before H+2 exists.
	_, err = s.verifyLightBlockAtHeight(ctx, height+2, time.Now())
	if err != nil {
		return nil, err
	}
	return header.AppHash, nil
}

// Commit implements StateProvider.
func (s *stateProviderP2P) Commit(ctx context.Context, height uint64) (*types.Commit, error) {
	s.Lock()
	defer s.Unlock()
	header, err := s.verifyLightBlockAtHeight(ctx, height, time.Now())
	if err != nil {
		return nil, err
	}
	return header.Commit, nil
}

// State implements StateProvider.
func (s *stateProviderP2P) State(ctx context.Context, height uint64) (sm.State, error) {
	s.Lock()
	defer s.Unlock()

	state := sm.State{
		ChainID:       s.lc.ChainID(),
		InitialHeight: s.initialHeight,
	}
	if state.InitialHeight == 0 {
		state.InitialHeight = 1
	}

	// The snapshot height maps onto the state heights as follows:
	//
	// height: last block, i.e. the snapshotted height
	// height+1: current block, i.e. the first block we'll process after the snapshot
	// height+2: next block, i.e. the second block after the snapshot
	//
	// We need to fetch the NextValidators from height+2 because if the application changed
	// the validator set at the snapshot height then this only takes effect at height+2.
	lastLightBlock, err := s.verifyLightBlockAtHeight(ctx, height, time.Now())
	if err != nil {
		return sm.State{}, err
	}
	currentLightBlock, err := s.verifyLightBlockAtHeight(ctx, height+1, time.Now())
	if err != nil {
		return sm.State{}, err
	}
	nextLightBlock, err := s.verifyLightBlockAtHeight(ctx, height+2, time.Now())
	if err != nil {
		return sm.State{}, err
	}

	state.Version = sm.Version{
		Consensus: currentLightBlock.Version,
		Software:  version.TMVersion,
	}
	state.LastBlockHeight = lastLightBlock.Height
	state.LastBlockTime = lastLightBlock.Time
	state.LastBlockID = lastLightBlock.Commit.BlockID
	state.AppHash = currentLightBlock.AppHash
	state.LastResultsHash = currentLightBlock.LastResultsHash
	state.LastValidators = lastLightBlock.ValidatorSet
	state.Validators = currentLightBlock.ValidatorSet
	state.NextValidators = nextLightBlock.ValidatorSet
	state.LastHeightValidatorsChanged = nextLightBlock.Height

	// We'll also need to fetch consensus params via P2P.
	state.ConsensusParams, err = s.consensusParams(ctx, currentLightBlock.Height)
	if err != nil {
		return sm.State{}, fmt.Errorf("fetching consensus params: %w", err)
	}
	// validate the consensus params
	if !bytes.Equal(nextLightBlock.ConsensusHash, state.ConsensusParams.HashConsensusParams()) {
		return sm.State{}, fmt.Errorf("consensus params hash mismatch at height %d. Expected %v, got %v",
			currentLightBlock.Height, nextLightBlock.ConsensusHash, state.ConsensusParams.HashConsensusParams())
	}
	// set the last height changed to the current height
	state.LastHeightConsensusParamsChanged = currentLightBlock.Height

	return state, nil
}

// addProvider dynamically adds a peer as a new witness. A limit of 6 providers is kept as a
// heuristic. Too many overburdens the network and too little compromises the second layer of security.
func (s *stateProviderP2P) addProvider(p lightprovider.Provider) {
	if len(s.lc.Witnesses()) < 6 {
		s.lc.AddProvider(p)
	}
}

// consensusParams sends out a request for consensus params blocking
// until one is returned.
//
// It attempts to send requests to all witnesses in parallel, but if
// none responds it will retry them all sometime later until it
// receives some response. This operation will block until it receives
// a response or the context is canceled.
func (s *stateProviderP2P) consensusParams(ctx context.Context, height int64) (types.ConsensusParams, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	out := make(chan types.ConsensusParams)

	retryAll := func() (<-chan struct{}, error) {
		wg := &sync.WaitGroup{}

		for _, provider := range s.lc.Witnesses() {
			p, ok := provider.(*BlockProvider)
			if !ok {
				return nil, fmt.Errorf("witness is not BlockProvider [%T]", provider)
			}

			peer, err := types.NewNodeID(p.String())
			if err != nil {
				return nil, fmt.Errorf("invalid provider (%s) node id: %w", p.String(), err)
			}

			wg.Add(1)
			go func(peer types.NodeID) {
				defer wg.Done()

				timer := time.NewTimer(0)
				defer timer.Stop()
				var iterCount int64

				for {
					iterCount++
					if err := s.paramsSendCh.Send(ctx, p2p.Envelope{
						To: peer,
						Message: &ssproto.ParamsRequest{
							Height: uint64(height),
						},
					}); err != nil {
						// this only errors if
						// the context is
						// canceled which we
						// don't need to
						// propagate here
						return
					}

					// jitter+backoff the retry loop
					timer.Reset(time.Duration(iterCount)*consensusParamsResponseTimeout +
						time.Duration(100*rand.Int63n(iterCount))*time.Millisecond) // nolint:gosec

					select {
					case <-timer.C:
						continue
					case <-ctx.Done():
						return
					case params, ok := <-s.paramsRecvCh:
						if !ok {
							return
						}
						select {
						case <-ctx.Done():
							return
						case out <- params:
							return
						}
					}
				}

			}(peer)
		}
		sig := make(chan struct{})
		go func() { wg.Wait(); close(sig) }()
		return sig, nil
	}

	timer := time.NewTimer(0)
	defer timer.Stop()

	var iterCount int64
	for {
		iterCount++
		sig, err := retryAll()
		if err != nil {
			return types.ConsensusParams{}, err
		}
		select {
		case <-sig:
			// jitter+backoff the retry loop
			timer.Reset(time.Duration(iterCount)*consensusParamsResponseTimeout +
				time.Duration(100*rand.Int63n(iterCount))*time.Millisecond) // nolint:gosec
			select {
			case param := <-out:
				return param, nil
			case <-ctx.Done():
				return types.ConsensusParams{}, ctx.Err()
			case <-timer.C:
			}
		case <-ctx.Done():
			return types.ConsensusParams{}, ctx.Err()
		case param := <-out:
			return param, nil
		}
	}

}
