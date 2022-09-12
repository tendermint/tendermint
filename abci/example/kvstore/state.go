package kvstore

import (
	"encoding/json"
	"errors"
	"fmt"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

type State interface {
	dbm.DB

	Save() error
	Load() error
	Copy(dst State) error
	Close() error

	GetHeight() int64

	GetAppHash() tmbytes.HexBytes
	UpdateAppHash(lastCommittedState State, txs [][]byte, txResults []*types.ExecTxResult) error
}

type kvState struct {
	dbm.DB
	Height  int64            `json:"height"`
	AppHash tmbytes.HexBytes `json:"app_hash"`
}

func NewKvState(db dbm.DB) State {
	return &kvState{
		DB:      db,
		AppHash: make([]byte, crypto.DefaultAppHashSize),
	}
}

// Copy copies the state. It ensures copy is a valid, initialized state.
// Caller should close the state once it's not needed anymore
// newDBfunc can be provided to define DB that will be used for this copy.
func (state kvState) Copy(destination State) error {
	dst, ok := destination.(*kvState)
	if !ok {
		return fmt.Errorf("invalid destination, expected: *kvState, got: %T", destination)
	}

	dst.Height = state.Height
	dst.AppHash = state.AppHash.Copy()
	// apphash is required, and should never be nil,zero-length
	if len(dst.AppHash) == 0 {
		dst.AppHash = make(tmbytes.HexBytes, crypto.DefaultAppHashSize)
	}
	if err := copyDB(state.DB, dst.DB); err != nil {
		return fmt.Errorf("copy state db: %w", err)
	}
	return nil
}

func copyDB(src dbm.DB, dst dbm.DB) error {
	dstBatch := dst.NewBatch()
	defer dstBatch.Close()

	// cleanup dest DB first
	dstIter, err := dst.Iterator(nil, nil)
	if err != nil {
		return fmt.Errorf("cannot create dest db iterator: %w", err)
	}
	defer dstIter.Close()

	// Delete content of dst, to be sure that it will not contain any unexpected data.
	keys := make([][]byte, 0)
	for dstIter.Valid() {
		keys = append(keys, dstIter.Key())
		dstIter.Next()
	}
	for _, key := range keys {
		_ = dstBatch.Delete(key) // ignore errors
	}

	// write source to dest
	if src != nil {
		srcIter, err := src.Iterator(nil, nil)
		if err != nil {
			return fmt.Errorf("cannot copy current DB: %w", err)
		}
		defer srcIter.Close()

		for srcIter.Valid() {
			if err = dstBatch.Set(srcIter.Key(), srcIter.Value()); err != nil {
				return err
			}
			srcIter.Next()
		}

		if err = dstBatch.Write(); err != nil {
			return fmt.Errorf("cannot close dest batch: %w", err)
		}
	}

	return nil
}

func (state kvState) GetHeight() int64 {
	return state.Height
}

func (state kvState) GetAppHash() tmbytes.HexBytes {
	return state.AppHash.Copy()
}
func (state *kvState) UpdateAppHash(lastCommittedState State, txs [][]byte, txResults []*types.ExecTxResult) error {
	// UpdateAppHash updates app hash for the current app state.
	txResultsHash, err := types.TxResultsHash(txResults)
	if err != nil {
		return err
	}
	state.AppHash = crypto.Checksum(append(lastCommittedState.GetAppHash(), txResultsHash...))

	return nil
}

func (state *kvState) Load() error {
	if state == nil || state.DB == nil {
		return errors.New("cannot load into nil state")
	}
	stateBytes, err := state.DB.Get(stateKey)
	if err != nil {
		return fmt.Errorf("kvState get: %w", err)
	}
	if len(stateBytes) == 0 {
		return nil // noop
	}
	err = json.Unmarshal(stateBytes, &state)
	if err != nil {
		return fmt.Errorf("kvState unmarshal: %w", err)
	}

	return nil
}

func (state kvState) Save() error {
	stateBytes, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("kvState marshal: %w", err)
	}
	err = state.DB.Set(stateKey, stateBytes)
	if err != nil {
		return fmt.Errorf("kvState set: %w", err)
	}

	return nil
}

func (state *kvState) Close() error {
	if state.DB != nil {
		return state.DB.Close()
	}
	return nil
}
