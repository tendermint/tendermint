package lite

import (
	"fmt"
	"regexp"
	"strconv"

	amino "github.com/tendermint/go-amino"
	cryptoAmino "github.com/tendermint/tendermint/crypto/encoding/amino"
	log "github.com/tendermint/tendermint/libs/log"
	lerr "github.com/tendermint/tendermint/lite/errors"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

var _ PersistentProvider = (*DBProvider)(nil)

// DBProvider stores commits and validator sets in a DB.
type DBProvider struct {
	logger log.Logger
	label  string
	db     dbm.DB
	cdc    *amino.Codec
	limit  int
}

func NewDBProvider(label string, db dbm.DB) *DBProvider {

	// NOTE: when debugging, this type of construction might be useful.
	//db = dbm.NewDebugDB("db provider "+cmn.RandStr(4), db)

	cdc := amino.NewCodec()
	cryptoAmino.RegisterAmino(cdc)
	dbp := &DBProvider{
		logger: log.NewNopLogger(),
		label:  label,
		db:     db,
		cdc:    cdc,
	}
	return dbp
}

func (dbp *DBProvider) SetLogger(logger log.Logger) {
	dbp.logger = logger.With("label", dbp.label)
}

func (dbp *DBProvider) SetLimit(limit int) *DBProvider {
	dbp.limit = limit
	return dbp
}

// Implements PersistentProvider.
func (dbp *DBProvider) SaveFullCommit(fc FullCommit) error {

	dbp.logger.Info("DBProvider.SaveFullCommit()...", "fc", fc)
	batch := dbp.db.NewBatch()
	defer batch.Close()

	// Save the fc.validators.
	// We might be overwriting what we already have, but
	// it makes the logic easier for now.
	vsKey := validatorSetKey(fc.ChainID(), fc.Height())
	vsBz, err := dbp.cdc.MarshalBinaryLengthPrefixed(fc.Validators)
	if err != nil {
		return err
	}
	batch.Set(vsKey, vsBz)

	// Save the fc.NextValidators.
	nvsKey := validatorSetKey(fc.ChainID(), fc.Height()+1)
	nvsBz, err := dbp.cdc.MarshalBinaryLengthPrefixed(fc.NextValidators)
	if err != nil {
		return err
	}
	batch.Set(nvsKey, nvsBz)

	// Save the fc.SignedHeader
	shKey := signedHeaderKey(fc.ChainID(), fc.Height())
	shBz, err := dbp.cdc.MarshalBinaryLengthPrefixed(fc.SignedHeader)
	if err != nil {
		return err
	}
	batch.Set(shKey, shBz)

	// And write sync.
	batch.WriteSync()

	// Garbage collect.
	// TODO: optimize later.
	if dbp.limit > 0 {
		dbp.deleteAfterN(fc.ChainID(), dbp.limit)
	}

	return nil
}

// Implements Provider.
func (dbp *DBProvider) LatestFullCommit(chainID string, minHeight, maxHeight int64) (
	FullCommit, error) {

	dbp.logger.Info("DBProvider.LatestFullCommit()...",
		"chainID", chainID, "minHeight", minHeight, "maxHeight", maxHeight)

	if minHeight <= 0 {
		minHeight = 1
	}
	if maxHeight == 0 {
		maxHeight = 1<<63 - 1
	}

	itr := dbp.db.ReverseIterator(
		signedHeaderKey(chainID, minHeight),
		append(signedHeaderKey(chainID, maxHeight), byte(0x00)),
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
			err := dbp.cdc.UnmarshalBinaryLengthPrefixed(shBz, &sh)
			if err != nil {
				return FullCommit{}, err
			} else {
				lfc, err := dbp.fillFullCommit(sh)
				if err == nil {
					dbp.logger.Info("DBProvider.LatestFullCommit() found latest.", "height", lfc.Height())
					return lfc, nil
				} else {
					dbp.logger.Error("DBProvider.LatestFullCommit() got error", "lfc", lfc)
					dbp.logger.Error(fmt.Sprintf("%+v", err))
					return lfc, err
				}
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
		err = lerr.ErrUnknownValidators(chainID, height)
		return
	}
	err = dbp.cdc.UnmarshalBinaryLengthPrefixed(vsBz, &valset)
	if err != nil {
		return
	}

	// To test deep equality.  This makes it easier to test for e.g. valset
	// equivalence using assert.Equal (tests for deep equality) in our tests,
	// which also tests for unexported/private field equivalence.
	valset.TotalVotingPower()

	return
}

func (dbp *DBProvider) fillFullCommit(sh types.SignedHeader) (FullCommit, error) {
	var chainID = sh.ChainID
	var height = sh.Height
	var valset, nextValset *types.ValidatorSet
	// Load the validator set.
	valset, err := dbp.getValidatorSet(chainID, height)
	if err != nil {
		return FullCommit{}, err
	}
	// Load the next validator set.
	nextValset, err = dbp.getValidatorSet(chainID, height+1)
	if err != nil {
		return FullCommit{}, err
	}
	// Return filled FullCommit.
	return FullCommit{
		SignedHeader:   sh,
		Validators:     valset,
		NextValidators: nextValset,
	}, nil
}

func (dbp *DBProvider) deleteAfterN(chainID string, after int) error {

	dbp.logger.Info("DBProvider.deleteAfterN()...", "chainID", chainID, "after", after)

	itr := dbp.db.ReverseIterator(
		signedHeaderKey(chainID, 1),
		append(signedHeaderKey(chainID, 1<<63-1), byte(0x00)),
	)
	defer itr.Close()

	var lastHeight int64 = 1<<63 - 1
	var numSeen = 0
	var numDeleted = 0

	for itr.Valid() {
		key := itr.Key()
		_, height, ok := parseChainKeyPrefix(key)
		if !ok {
			return fmt.Errorf("unexpected key %v", key)
		} else {
			if height < lastHeight {
				lastHeight = height
				numSeen += 1
			}
			if numSeen > after {
				dbp.db.Delete(key)
				numDeleted += 1
			}
		}
		itr.Next()
	}

	dbp.logger.Info(fmt.Sprintf("DBProvider.deleteAfterN() deleted %v items", numDeleted))
	return nil
}

//----------------------------------------
// key encoding

func signedHeaderKey(chainID string, height int64) []byte {
	return []byte(fmt.Sprintf("%s/%010d/sh", chainID, height))
}

func validatorSetKey(chainID string, height int64) []byte {
	return []byte(fmt.Sprintf("%s/%010d/vs", chainID, height))
}

//----------------------------------------
// key parsing

var keyPattern = regexp.MustCompile(`^([^/]+)/([0-9]*)/(.*)$`)

func parseKey(key []byte) (chainID string, height int64, part string, ok bool) {
	submatch := keyPattern.FindSubmatch(key)
	if submatch == nil {
		return "", 0, "", false
	}
	chainID = string(submatch[1])
	heightStr := string(submatch[2])
	heightInt, err := strconv.Atoi(heightStr)
	if err != nil {
		return "", 0, "", false
	}
	height = int64(heightInt)
	part = string(submatch[3])
	ok = true // good!
	return
}

func parseSignedHeaderKey(key []byte) (chainID string, height int64, ok bool) {
	var part string
	chainID, height, part, ok = parseKey(key)
	if part != "sh" {
		return "", 0, false
	}
	return
}

func parseChainKeyPrefix(key []byte) (chainID string, height int64, ok bool) {
	chainID, height, _, ok = parseKey(key)
	return
}
