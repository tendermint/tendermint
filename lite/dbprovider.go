package lite

import (
	"fmt"
	"regexp"
	"strconv"

	amino "github.com/tendermint/go-amino"
	crypto "github.com/tendermint/go-crypto"
	lerr "github.com/tendermint/tendermint/lite/errors"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tmlibs/db"
)

func signedHeaderKey(chainID string, height int64) []byte {
	return []byte(fmt.Sprintf("%s/%010d/sh", chainID, height))
}

var signedHeaderKeyPattern = regexp.MustCompile(`([^/]+)/([0-9]*)/sh`)

func parseSignedHeaderKey(key []byte) (chainID string, height int64, ok bool) {
	submatch := signedHeaderKeyPattern.FindSubmatch(key)
	if submatch == nil {
		return "", 0, false
	}
	chainID = string(submatch[1])
	heightStr := string(submatch[2])
	heightInt, err := strconv.Atoi(heightStr)
	if err != nil {
		return "", 0, false
	}
	height = int64(heightInt)
	ok = true // good!
	return
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
	//db = dbm.NewDebugDB("db provider "+cmn.RandStr(4), db)
	cdc := amino.NewCodec()
	crypto.RegisterAmino(cdc)
	dbp := &DBProvider{db: db, cdc: cdc}
	return dbp
}

// Implements PersistentProvider.
func (dbp *DBProvider) SaveFullCommit(fc FullCommit) error {

	batch := dbp.db.NewBatch()

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

	for itr.Valid() {
		key := itr.Key()
		_, _, ok := parseSignedHeaderKey(key)
		if !ok {
			// Skip over other keys.
			itr.Next()
			continue
		} else {
			// Found the latest full commit signed header.
			shBz := itr.Value()
			sh := types.SignedHeader{}
			err := dbp.cdc.UnmarshalBinary(shBz, &sh)
			if err != nil {
				return FullCommit{}, err
			} else {
				return dbp.fillFullCommit(sh)
			}
		}
	}
	return FullCommit{}, lerr.ErrCommitNotFound()
}

func (dbp *DBProvider) ValidatorSet(chainID string, height int64) (valset *types.ValidatorSet, err error) {
	return dbp.getValidatorSet(chainID, height)
}

func (dbp *DBProvider) getValidatorSet(chainID string, height int64) (valset *types.ValidatorSet, err error) {
	vsBz := dbp.db.Get(validatorSetKey(chainID, height))
	if vsBz == nil {
		err = lerr.ErrMissingValidators(chainID, height)
		return
	}
	err = dbp.cdc.UnmarshalBinary(vsBz, &valset)
	if err != nil {
		return
	}
	valset.TotalVotingPower() // to test deep equality.
	return
}

func (dbp *DBProvider) fillFullCommit(sh types.SignedHeader) (FullCommit, error) {
	var chainID = sh.ChainID
	var height = sh.Height
	var valset, nvalset *types.ValidatorSet
	// Load the validator set.
	valset, err := dbp.getValidatorSet(chainID, height)
	if err != nil {
		return FullCommit{}, err
	}
	// Load the next validator set.
	nvalset, err = dbp.getValidatorSet(chainID, height+1)
	if err != nil {
		return FullCommit{}, err
	}
	// Return filled FullCommit.
	return FullCommit{
		SignedHeader:   sh,
		Validators:     valset,
		NextValidators: nvalset,
	}, nil
}
