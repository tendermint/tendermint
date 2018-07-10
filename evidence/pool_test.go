package evidence

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tendermint/libs/db"
)

var mockState = sm.State{}

func initializeValidatorState(valAddr []byte, height int64) dbm.DB {
	stateDB := dbm.NewMemDB()

	// create validator set and state
	valSet := &types.ValidatorSet{
		Validators: []*types.Validator{
			{Address: valAddr},
		},
	}
	state := sm.State{
		LastBlockHeight:             0,
		LastBlockTime:               time.Now(),
		Validators:                  valSet,
		LastHeightValidatorsChanged: 1,
		ConsensusParams: types.ConsensusParams{
			EvidenceParams: types.EvidenceParams{
				MaxAge: 1000000,
			},
		},
	}

	// save all states up to height
	for i := int64(0); i < height; i++ {
		state.LastBlockHeight = i
		sm.SaveState(stateDB, state)
	}

	return stateDB
}

func TestEvidencePool(t *testing.T) {

	valAddr := []byte("val1")
	height := int64(5)
	stateDB := initializeValidatorState(valAddr, height)
	store := NewEvidenceStore(dbm.NewMemDB())
	pool := NewEvidencePool(stateDB, store)

	goodEvidence := types.NewMockGoodEvidence(height, 0, valAddr)
	badEvidence := types.MockBadEvidence{goodEvidence}

	// bad evidence
	err := pool.AddEvidence(badEvidence)
	assert.NotNil(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		<-pool.EvidenceWaitChan()
		wg.Done()
	}()

	err = pool.AddEvidence(goodEvidence)
	assert.Nil(t, err)
	wg.Wait()

	assert.Equal(t, 1, pool.evidenceList.Len())

	// if we send it again, it shouldnt change the size
	err = pool.AddEvidence(goodEvidence)
	assert.Nil(t, err)
	assert.Equal(t, 1, pool.evidenceList.Len())
}
