package files

import (
	"encoding/json"
	"os"

	"github.com/pkg/errors"

	wire "github.com/tendermint/go-wire"

	"github.com/tendermint/tendermint/lite"
	liteErr "github.com/tendermint/tendermint/lite/errors"
)

const (
	// MaxFullCommitSize is the maximum number of bytes we will
	// read in for a full commit to avoid excessive allocations
	// in the deserializer
	MaxFullCommitSize = 1024 * 1024
)

// SaveFullCommit exports the seed in binary / go-wire style
func SaveFullCommit(fc lite.FullCommit, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.Close()

	var n int
	wire.WriteBinary(fc, f, &n, &err)
	return errors.WithStack(err)
}

// SaveFullCommitJSON exports the seed in a json format
func SaveFullCommitJSON(fc lite.FullCommit, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.Close()
	stream := json.NewEncoder(f)
	err = stream.Encode(fc)
	return errors.WithStack(err)
}

// LoadFullCommit loads the full commit from the file system.
func LoadFullCommit(path string) (lite.FullCommit, error) {
	var fc lite.FullCommit
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fc, liteErr.ErrCommitNotFound()
		}
		return fc, errors.WithStack(err)
	}
	defer f.Close()

	var n int
	wire.ReadBinaryPtr(&fc, f, MaxFullCommitSize, &n, &err)
	return fc, errors.WithStack(err)
}

// LoadFullCommitJSON loads the commit from the file system in JSON format.
func LoadFullCommitJSON(path string) (lite.FullCommit, error) {
	var fc lite.FullCommit
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fc, liteErr.ErrCommitNotFound()
		}
		return fc, errors.WithStack(err)
	}
	defer f.Close()

	stream := json.NewDecoder(f)
	err = stream.Decode(&fc)
	return fc, errors.WithStack(err)
}
