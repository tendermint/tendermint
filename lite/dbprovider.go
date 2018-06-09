package lite

import (
	"encoding/hex"
	"sort"
	"sync"

	amino "github.com/tendermint/go-amino"
	crypto "github.com/tendermint/go-crypto"
	lerr "github.com/tendermint/tendermint/lite/errors"
	dbm "github.com/tendermint/tmlibs/db"
)

func signedHeaderKey(chainID string, height int64) []byte {
	return []byte(fmt.Sprintf("%s/%010d/sh", chainID, height))
}

func validatorSetKey(chainID string, height int64) []byte {
	return []byte(fmt.Sprintf("%s/%010d/vs", chainID, height))
}

type DBProvider struct {
	chainID string
	db      dbm.DB
	cdc     *amino.Codec
}

func NewDBProvider(db dbm.DB) *DBProvider {
	cdc := amino.NewCodec()
	crypto.RegisterAmino(cdC)
	dbp := &DBProvider{db: db, cdc: cdc}
	return dbp
}

// Implements PersistentProvider.
func (dbp *DBProvider) SaveFullCommit(fc FullCommit) error {

	batch := dbp.db.Batch()

	// Save the fc.validators.
	// We might be overwriting what we already have, but
	// it makes the logic easier for now.
	vsKey := validatorSetKey(fc.ChainID(), fc.Height())
	vsBz, err := dbp.cdc.MarshalBinary(fc.Validators)
	if err != nil {
		return err
	}
	batch.Set(vsKey, vsBz)

	// Save the fc.NextValidators.
	nvsKey := validatorSetKey(fc.ChainID(), fc.Height()+1)
	nvsBz, err := dbp.cdc.MarshalBinary(fc.NextValidators)
	if err != nil {
		return err
	}
	batch.Set(nvsKey, nvsBz)

	// Save the fc.SignedHeader
	shKey := signedHeaderKey(fc.ChainID(), fc.Height())
	shBz, err := dbp.cdc.MarshalBinary(fc.SignedHeader)
	if err != nil {
		return err
	}
	batch.Set(shKey, shBz)

	// And write sync.
	batch.WriteSync()
	return nil
}

// Implements Provider.
func (dbp *DBProvider) LatestFullCommit(chainID string, minHeight, maxHeight int64) (
	FullCommit, error) {

	if minHeight <= 0 {
		minHeight = 1
	}
	if maxHeight == 0 {
		maxHeight = 1<<63 - 1
	}

	itr := dbp.db.ReverseIterator(
		signedHeaderKey(chainID, maxHeight),
		signedHeaderKey(chainID, minHeight-1),
	)
	defer itr.Close()

	if itr.Valid() {
		// Found the latest full commit signed header.
		shBz := itr.Value()
		sh := SignedHeader{}
		err := dbp.cdc.UnmarshalBinary(shBz, &sh)
		if err != nil {
			return FullCommit{}, nil
		} else {
			return dbp.fillFullCommit(sh)
		}
	} else {
		return FullCommit{}, lerr.ErrCommitNotFound()
	}
}

func (dbp *DBProvider) fillFullCommit(sh *types.SignedHeader) (FullCommit, error) {
	var chainID = sh.Header.ChainID
	var height = sh.Header.Height
	var valset, nvalset *types.ValidatorSet
	// Load the validator set.
	vsBz := dbp.db.Get(validatorSetKey(chainID, sh.Header.Height))
	if vsBz == nil {
		return FullCommit{}, errors.New("missing validator set at DB key %s",
			validatorSetKey(chainID, sh.Header.Height))
	}
	err := dbp.cdc.UnmarshalBinary(vsBz, &valset)
	if err != nil {
		return FullCommit{}, nil
	}
	// Load the next validator set.
	nvsBz := dbp.db.Get(validatorSetKey(chainID, sh.Header.Height+1))
	if vsBz == nil {
		return FullCommit{}, errors.New("missing validator set at DB key %s",
			validatorSetKey(chainID, sh.Header.Height+1))
	}
	err = dbp.cdc.UnmarshalBinary(nvsBz, &nvalset)
	if err != nil {
		return FullCommit{}, nil
	}
	return FullCommit{
		SignedHeader:   sh,
		Validators:     valset,
		NextValidators: nvalset,
	}, nil
}
