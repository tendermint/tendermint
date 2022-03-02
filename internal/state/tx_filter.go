package state

import (
	"sync"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/types"
)

func cachingStateFetcher(store Store) func() (State, error) {
	var (
		last  time.Time
		mutex = &sync.Mutex{}
		ttl   time.Duration
		cache State
		err   error
	)

	return func() (State, error) {
		mutex.Lock()
		defer mutex.Unlock()

		if time.Since(last) < ttl && cache.ChainID != "" {
			return cache, nil
		}

		cache, err = store.Load()
		if err != nil {
			return State{}, err
		}
		last = time.Now()
		// at least 100ms but maybe as much as that+
		// a block interval. This might end up being
		// too much, but it replaces a mechanism that
		// cached these values for the entire runtime
		// of the process
		ttl = (100 * time.Millisecond) + cache.LastBlockTime.Sub(last)

		return cache, nil
	}

}

// TxPreCheck returns a function to filter transactions before processing.
// The function limits the size of a transaction to the block's maximum data size.
func TxPreCheck(store Store) mempool.PreCheckFunc {
	fetch := cachingStateFetcher(store)

	return func(tx types.Tx) error {
		state, err := fetch()
		if err != nil {
			return err
		}

		return TxPreCheckForState(state)(tx)
	}
}

func TxPreCheckForState(state State) mempool.PreCheckFunc {
	return func(tx types.Tx) error {
		maxDataBytes := types.MaxDataBytesNoEvidence(
			state.ConsensusParams.Block.MaxBytes,
			state.Validators.Size(),
		)
		return mempool.PreCheckMaxBytes(maxDataBytes)(tx)
	}

}

// TxPostCheck returns a function to filter transactions after processing.
// The function limits the gas wanted by a transaction to the block's maximum total gas.
func TxPostCheck(store Store) mempool.PostCheckFunc {
	fetch := cachingStateFetcher(store)

	return func(tx types.Tx, resp *abci.ResponseCheckTx) error {
		state, err := fetch()
		if err != nil {
			return err
		}
		return mempool.PostCheckMaxGas(state.ConsensusParams.Block.MaxGas)(tx, resp)
	}
}

func TxPostCheckForState(state State) mempool.PostCheckFunc {
	return func(tx types.Tx, resp *abci.ResponseCheckTx) error {
		return mempool.PostCheckMaxGas(state.ConsensusParams.Block.MaxGas)(tx, resp)
	}
}
