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
func New(db dbm.DB, prefix string) store.Store {
	cdc := amino.NewCodec()
	cryptoAmino.RegisterAmino(cdc)
	return &dbs{db: db, prefix: prefix, cdc: cdc}
}

func (s *dbs) SaveSignedHeader(sh *types.SignedHeader) error {
	if sh.Height <= 0 {
		panic("negative or zero height")
	}

	bz, err := s.cdc.MarshalBinaryLengthPrefixed(sh)
	if err != nil {
		return err
	}
	s.db.Set(s.shKey(sh.Height), bz)
	return nil
}

func (s *dbs) SaveValidatorSet(valSet *types.ValidatorSet, height int64) error {
	if height <= 0 {
		panic("negative or zero height")
	}

	bz, err := s.cdc.MarshalBinaryLengthPrefixed(valSet)
	if err != nil {
		return err
	}
	s.db.Set(s.vsKey(height), bz)
	return nil
}

func (s *dbs) SignedHeader(height int64) (*types.SignedHeader, error) {
	bz := s.db.Get(s.shKey(height))
	if bz == nil {
		return nil, nil
	}

	var signedHeader *types.SignedHeader
	err := s.cdc.UnmarshalBinaryLengthPrefixed(bz, &signedHeader)
	return signedHeader, err
}

func (s *dbs) ValidatorSet(height int64) (*types.ValidatorSet, error) {
	bz := s.db.Get(s.vsKey(height))
	if bz == nil {
		return nil, nil
	}

	var valSet *types.ValidatorSet
	err := s.cdc.UnmarshalBinaryLengthPrefixed(bz, &valSet)
	return valSet, err
}

func (s *dbs) LastSignedHeaderHeight() (int64, error) {
	itr := s.db.ReverseIterator(
		s.shKey(1),
		append(s.shKey(1<<63-1), byte(0x00)),
	)
	defer itr.Close()

	for itr.Valid() {
		key := itr.Key()
		_, height, ok := parseShKey(key)
		if ok {
			return height, nil
		}
	}

	return -1, errors.New("no headers found")
}

func (s *dbs) shKey(height int64) []byte {
	return []byte(fmt.Sprintf("sh/%s/%010d", s.prefix, height))
}

func (s *dbs) vsKey(height int64) []byte {
	return []byte(fmt.Sprintf("vs/%s/%010d", s.prefix, height))
}

var keyPattern = regexp.MustCompile(`^(sh|vs)/([^/]*)/([0-9]+)/$`)

func parseKey(key []byte) (part string, prefix string, height int64, ok bool) {
	submatch := keyPattern.FindSubmatch(key)
	if submatch == nil {
		return "", "", 0, false
	}
	part = string(submatch[1])
	prefix = string(submatch[2])
	heightStr := string(submatch[3])
	heightInt, err := strconv.Atoi(heightStr)
	if err != nil {
		return "", "", 0, false
	}
	height = int64(heightInt)
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
