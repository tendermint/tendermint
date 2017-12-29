package evidence

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tmlibs/db"
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
	assert := assert.New(t)

	valAddr := []byte("val1")
	height := int64(5)
	stateDB := initializeValidatorState(valAddr, height)
	store := NewEvidenceStore(dbm.NewMemDB())
	pool := NewEvidencePool(stateDB, store)

	goodEvidence := types.NewMockGoodEvidence(height, 0, valAddr)
	badEvidence := types.MockBadEvidence{goodEvidence}

	err := pool.AddEvidence(badEvidence)
	assert.NotNil(err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		<-pool.EvidenceChan()
		wg.Done()
	}()

	err = pool.AddEvidence(goodEvidence)
	assert.Nil(err)
	wg.Wait()

	// if we send it again it wont fire on the chan
	err = pool.AddEvidence(goodEvidence)
	assert.Nil(err)
	select {
	case <-pool.EvidenceChan():
		t.Fatal("unexpected read on EvidenceChan")
	default:
	}
}
