package kvstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

type State interface {
	dbm.DB

	Save(to io.Writer) error
	Load(from io.Reader) error
	Copy(dst State) error
	Close() error

	GetHeight() int64

	NextHeightState(db dbm.DB) (State, error)

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

// NextHeightState creates a state at next height with a copy of all key/value pairs.
// It uses `db` as a backend database.
func (state kvState) NextHeightState(db dbm.DB) (State, error) {
	height := state.GetHeight() + 1
	nextState := NewKvState(db).(*kvState)
	err := state.Copy(nextState)
	nextState.Height = height
	if err != nil {
		return &kvState{}, fmt.Errorf("cannot copy current state: %w", err)
	}
	// overwrite what was set in Copy, as we are at new height
	nextState.Height = height
	return nextState, nil
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

func (state *kvState) Close() error {
	if state.DB != nil {
		return state.DB.Close()
	}
	return nil
}

// StateReader is a wrapper around dbm.DB that provides io.Reader to read a state.
// Note that you should create a new StateReaderWriter each time you use it.
type StateReaderWriter struct {
	dbm.DB
	data *bytes.Buffer
}

// Read implements io.Reader
func (w *StateReaderWriter) Read(p []byte) (n int, err error) {
	if w.data == nil {
		data, err := w.DB.Get(stateKey)
		if err != nil {
			return 0, err
		}
		w.data = bytes.NewBuffer(data)
	}

	return w.data.Read(p)
}

// Write implements io.Writer
func (w *StateReaderWriter) Write(p []byte) (int, error) {
	if err := w.DB.Set(stateKey, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *StateReaderWriter) Close() error {
	return w.DB.Close()
}
