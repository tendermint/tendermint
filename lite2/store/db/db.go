package db

import (
	"fmt"

	"github.com/tendermint/go-amino"
	dbm "github.com/tendermint/tm-db"

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
	return &dbs{db: db, prefix: prefix, cdc: amino.NewCodec()}
}

func (s *dbs) SaveSignedHeader(sh *types.SignedHeader) error {
	bz, err := s.cdc.MarshalBinaryLengthPrefixed(sh)
	if err != nil {
		return err
	}
	s.db.Set(s.shKey(sh.Height), bz)
	return nil
}

func (s *dbs) SaveValidatorSet(valSet *types.ValidatorSet, height int64) error {
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

func (s *dbs) shKey(height int64) []byte {
	return []byte(fmt.Sprintf("%s:SH:%v", s.prefix, height))
}

func (s *dbs) vsKey(height int64) []byte {
	return []byte(fmt.Sprintf("%s:VS:%v", s.prefix, height))
}
