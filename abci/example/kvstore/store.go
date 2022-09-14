package kvstore

import (
	"bytes"
	"io"

	dbm "github.com/tendermint/tm-db"
)

type Store interface {
	io.ReadWriteCloser
}

// dbStore is a wrapper around dbm.DB that provides io.Reader to read a state.
// Note that you should create a new StateReaderWriter each time you use it.
type dbStore struct {
	dbm.DB
	buf *bytes.Buffer
}

func NewDBStateStore(db dbm.DB) Store {
	return &dbStore{DB: db}
}

func (w *dbStore) key() []byte {
	return []byte(storeKey)
}

// Read implements io.Reader
func (w *dbStore) Read(p []byte) (n int, err error) {
	if w.buf == nil {
		data, err := w.DB.Get(w.key())
		if err != nil {
			return 0, err
		}
		w.buf = bytes.NewBuffer(data)
	}

	return w.buf.Read(p)
}

// Write implements io.Writer
func (w *dbStore) Write(p []byte) (int, error) {
	if err := w.DB.Set(w.key(), p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close implements io.Closer
func (w *dbStore) Close() error {
	return w.DB.Close()
}
