package evidence

import (
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

//go:generate mockery --case underscore --name BlockStore

type BlockStore interface {
	LoadBlockMeta(height int64) *types.BlockMeta
}

type StateStore interface {
	LoadValidators(height int64) (*types.ValidatorSet, error)
	LoadState() state.State
}

type stateStore struct {
	db dbm.DB
}

var _ StateStore = &stateStore{}

// This is a temporary measure until stateDB becomes a store
// TODO: deprecate once state has a store
func NewEvidenceStateStore(db dbm.DB) StateStore {
	return &stateStore{db}
}

func (s *stateStore) LoadValidators(height int64) (*types.ValidatorSet, error) {
	return state.LoadValidators(s.db, height)
}

func (s *stateStore) LoadState() state.State {
	return state.LoadState(s.db)
}
