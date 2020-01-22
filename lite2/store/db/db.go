package db

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/tendermint/go-amino"
	dbm "github.com/tendermint/tm-db"

	cryptoAmino "github.com/tendermint/tendermint/crypto/encoding/amino"
	"github.com/tendermint/tendermint/lite2/store"
	"github.com/tendermint/tendermint/types"
)

type dbs struct {
	db     dbm.DB
	prefix string

	cdc *amino.Codec
}

// New returns a Store that wraps any DB (with an optional prefix in case you
// want to use one DB with many light clients).
//
// Objects are marshalled using amino (github.com/tendermint/go-amino)
func New(db dbm.DB, prefix string) store.Store {
	cdc := amino.NewCodec()
	cryptoAmino.RegisterAmino(cdc)
	return &dbs{db: db, prefix: prefix, cdc: cdc}
}

// SaveSignedHeaderAndNextValidatorSet persists SignedHeader and ValidatorSet
// to the db.
func (s *dbs) SaveSignedHeaderAndNextValidatorSet(sh *types.SignedHeader, valSet *types.ValidatorSet) error {
	if sh.Height <= 0 {
		panic("negative or zero height")
	}

	// TODO: batch
	bz, err := s.cdc.MarshalBinaryLengthPrefixed(sh)
	if err != nil {
		return err
	}
	s.db.Set(s.shKey(sh.Height), bz)

	bz, err = s.cdc.MarshalBinaryLengthPrefixed(valSet)
	if err != nil {
		return err
	}
	s.db.Set(s.vsKey(sh.Height+1), bz)

	return nil
}

// DeleteSignedHeaderAndNextValidatorSet deletes SignedHeader and ValidatorSet
// from the db.
func (s *dbs) DeleteSignedHeaderAndNextValidatorSet(height int64) error {
	if height <= 0 {
		panic("negative or zero height")
	}

	// TODO: batch
	s.db.Delete(s.shKey(height))
	s.db.Delete(s.vsKey(height + 1))

	return nil
}

// SignedHeader loads SignedHeader at the given height.
func (s *dbs) SignedHeader(height int64) (*types.SignedHeader, error) {
	if height <= 0 {
		panic("negative or zero height")
	}

	bz, err := s.db.Get(s.shKey(height))
	if err != nil {
		panic(err)
	}
	if len(bz) == 0 {
		return nil, errors.New("signed header not found")
	}

	var signedHeader *types.SignedHeader
	err = s.cdc.UnmarshalBinaryLengthPrefixed(bz, &signedHeader)
	return signedHeader, err
}

// ValidatorSet loads ValidatorSet at the given height.
func (s *dbs) ValidatorSet(height int64) (*types.ValidatorSet, error) {
	if height <= 0 {
		panic("negative or zero height")
	}

	bz, err := s.db.Get(s.vsKey(height))
	if err != nil {
		panic(err)
	}
	if len(bz) == 0 {
		return nil, errors.New("validator set not found")
	}

	var valSet *types.ValidatorSet
	err = s.cdc.UnmarshalBinaryLengthPrefixed(bz, &valSet)
	return valSet, err
}

// LastSignedHeaderHeight returns the last SignedHeader height stored.
func (s *dbs) LastSignedHeaderHeight() (int64, error) {
	itr, err := s.db.ReverseIterator(
		s.shKey(1),
		append(s.shKey(1<<63-1), byte(0x00)),
	)
	if err != nil {
		panic(err)
	}
	defer itr.Close()

	for itr.Valid() {
		key := itr.Key()
		_, height, ok := parseShKey(key)
		if ok {
			return height, nil
		}
		itr.Next()
	}

	return -1, nil
}

// FirstSignedHeaderHeight returns the first SignedHeader height stored.
func (s *dbs) FirstSignedHeaderHeight() (int64, error) {
	itr, err := s.db.Iterator(
		s.shKey(1),
		append(s.shKey(1<<63-1), byte(0x00)),
	)
	if err != nil {
		panic(err)
	}
	defer itr.Close()

	for itr.Valid() {
		key := itr.Key()
		_, height, ok := parseShKey(key)
		if ok {
			return height, nil
		}
		itr.Next()
	}

	return -1, nil
}

func (s *dbs) shKey(height int64) []byte {
	return []byte(fmt.Sprintf("sh/%s/%020d", s.prefix, height))
}

func (s *dbs) vsKey(height int64) []byte {
	return []byte(fmt.Sprintf("vs/%s/%020d", s.prefix, height))
}

var keyPattern = regexp.MustCompile(`^(sh|vs)/([^/]*)/([0-9]+)$`)

func parseKey(key []byte) (part string, prefix string, height int64, ok bool) {
	submatch := keyPattern.FindSubmatch(key)
	if submatch == nil {
		return "", "", 0, false
	}
	part = string(submatch[1])
	prefix = string(submatch[2])
	height, err := strconv.ParseInt(string(submatch[3]), 10, 64)
	if err != nil {
		return "", "", 0, false
	}
	ok = true // good!
	return
}

func parseShKey(key []byte) (prefix string, height int64, ok bool) {
	var part string
	part, prefix, height, ok = parseKey(key)
	if part != "sh" {
		return "", 0, false
	}
	return
}
