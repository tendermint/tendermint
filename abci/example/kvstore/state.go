package kvstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	db "github.com/tendermint/tm-db"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

type State interface {
	dbm.DB
	json.Marshaler
	json.Unmarshaler

	Save(to io.Writer) error
	Load(from io.Reader) error
	Copy(dst State) error
	Close() error

	GetHeight() int64
	incHeight() // IncHeight increments height by 1

	NextHeightState(db dbm.DB) (State, error)

	GetAppHash() tmbytes.HexBytes
	UpdateAppHash(lastCommittedState State, txs []*types.TxRecord, txResults []*types.ExecTxResult) error
}

type kvState struct {
	dbm.DB
	Height  int64            `json:"height"`
	AppHash tmbytes.HexBytes `json:"app_hash"`
}

// NewKvState creates new, empty, uninitialized kvstore State.
// Use Copy() to populate.
func NewKvState(db dbm.DB, height int64) State {
	return &kvState{
		DB:      db,
		AppHash: make([]byte, crypto.DefaultAppHashSize),
		Height:  height,
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

func resetDB(dst dbm.DB, batch db.Batch) error {
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
		_ = batch.Delete(key) // ignore errors
	}

	return nil
}
func copyDB(src dbm.DB, dst dbm.DB) error {
	dstBatch := dst.NewBatch()
	defer dstBatch.Close()

	if err := resetDB(dst, dstBatch); err != nil {
		return err
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

func (state *kvState) incHeight() {
	state.Height++
}

// NextHeightState creates a state at next height with a copy of all key/value pairs.
// It uses `db` as a backend database.
func (state kvState) NextHeightState(db dbm.DB) (State, error) {
	nextState := NewKvState(db, 1)
	err := state.Copy(nextState)
	if err != nil {
		return &kvState{}, fmt.Errorf("cannot copy current state: %w", err)
	}
	nextState.incHeight() // height is overwritten in Copy()

	return nextState, nil
}

func (state kvState) GetAppHash() tmbytes.HexBytes {
	return state.AppHash.Copy()
}

func (state *kvState) UpdateAppHash(lastCommittedState State, txs []*types.TxRecord, txResults []*types.ExecTxResult) error {
	// UpdateAppHash updates app hash for the current app state.
	txResultsHash, err := types.TxResultsHash(txResults)
	if err != nil {
		return err
	}
	state.AppHash = crypto.Checksum(append(lastCommittedState.GetAppHash(), txResultsHash...))

	return nil
}

func (state *kvState) Load(from io.Reader) error {
	if state == nil || state.DB == nil {
		return errors.New("cannot load into nil state")
	}

	stateBytes, err := ioutil.ReadAll(from)
	if err != nil {
		return fmt.Errorf("kvState read: %w", err)
	}
	if len(stateBytes) == 0 {
		return nil // NOOP
	}

	err = json.Unmarshal(stateBytes, &state)
	if err != nil {
		return fmt.Errorf("kvState unmarshal: %w", err)
	}

	return nil
}

func (state kvState) Save(to io.Writer) error {
	stateBytes, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("kvState marshal: %w", err)
	}

	_, err = to.Write(stateBytes)
	if err != nil {
		return fmt.Errorf("kvState write: %w", err)
	}

	return nil
}
func (state kvState) Import(height uint64, jsonBytes []byte) error {
	return fmt.Errorf("not implemented")
}

type StateExport struct {
	Height  *int64            `json:"height,omitempty"`
	AppHash tmbytes.HexBytes  `json:"app_hash,omitempty"`
	Items   map[string]string `json:"items,omitempty"` // we store items as string-encoded values
}

// MarshalJSON implements json.Marshaler
func (state kvState) MarshalJSON() ([]byte, error) {
	iter, err := state.DB.Iterator(nil, nil)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	height := state.GetHeight()
	apphash := state.GetAppHash()

	export := StateExport{
		Height:  &height,
		AppHash: apphash,
		Items:   nil,
	}

	for iter.Valid() {
		if export.Items == nil {
			export.Items = map[string]string{}
		}
		export.Items[string(iter.Key())] = string(iter.Value())
		iter.Next()
	}

	return json.Marshal(&export)
}

// UnmarshalJSON implements json.Unmarshaler.
// Note that it unmarshals only existing (non-nil) values.
// If unmarshaled data contains a nil value (eg. is not present in json), these will stay intact.
func (state *kvState) UnmarshalJSON(data []byte) error {

	export := StateExport{}
	if err := json.Unmarshal(data, &export); err != nil {
		return err
	}

	if export.Height != nil {
		state.Height = *export.Height
	}
	if export.AppHash != nil {
		state.AppHash = export.AppHash
	}

	if export.Items != nil {
		batch := state.DB.NewBatch()
		defer batch.Close()

		if len(export.Items) > 0 {
			if err := resetDB(state.DB, batch); err != nil {
				return err
			}
			for key, value := range export.Items {
				if err := batch.Set([]byte(key), []byte(value)); err != nil {
					return err
				}
			}
		}
		if err := batch.Write(); err != nil {
			return err
		}
	}

	return nil
}

func (state *kvState) Close() error {
	if state.DB != nil {
		return state.DB.Close()
	}
	return nil
}
