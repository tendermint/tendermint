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

		if cache.ChainID == "" { // is nil
			cache, err = store.Load()
			if err != nil {
				return State{}, err
			}
			last = time.Now()
			ttl = 100*time.Millisecond + (cache.LastBlockTime.Add(last) * 2)
		}

	}

}

// TxPreCheck returns a function to filter transactions before processing.
// The function limits the size of a transaction to the block's maximum data size.
func TxPreCheck(store Store) mempool.PreCheckFunc {
	return func(tx types.Tx) error {
		// TODO: this should probably be cached at some level.
		state, err := store.Load()
		if err != nil {
			return err
		}
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
	return func(tx types.Tx, resp *abci.ResponseCheckTx) error {
		// TODO: this should probably be cached at some level.
		state, err := store.Load()
		if err != nil {
			return err
		}
		return mempool.PostCheckMaxGas(state.ConsensusParams.Block.MaxGas)(tx, resp)
	}
}
