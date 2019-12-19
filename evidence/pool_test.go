package evidence

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
	dbm "github.com/tendermint/tm-db"
)

func TestMain(m *testing.M) {
	types.RegisterMockEvidences(cdc)

	code := m.Run()
	os.Exit(code)
}

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
		LastBlockTime:               tmtime.Now(),
		Validators:                  valSet,
		NextValidators:              valSet.CopyIncrementProposerPriority(1),
		LastHeightValidatorsChanged: 1,
		ConsensusParams: types.ConsensusParams{
			Evidence: types.EvidenceParams{
				MaxAgeNumBlocks: 1000000,
				MaxAgeDuration:  48 * time.Hour,
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
	evidenceDB := dbm.NewMemDB()
	pool := NewPool(stateDB, evidenceDB)

	goodEvidence := types.NewMockGoodEvidence(height, 0, valAddr)
	badEvidence := types.NewMockBadEvidence(height, time.Now(), 0, valAddr)

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

func TestEvidencePoolIsCommitted(t *testing.T) {
	// Initialization:
	var (
		valAddr       = []byte("validator_address")
		height        = int64(42)
		lastBlockTime = time.Now()
		stateDB       = initializeValidatorState(valAddr, height)
		evidenceDB    = dbm.NewMemDB()
		pool          = NewPool(stateDB, evidenceDB)
	)

	// evidence not seen yet:
	evidence := types.NewMockGoodEvidence(height, 0, valAddr)
	assert.False(t, pool.IsCommitted(evidence))

	// evidence seen but not yet committed:
	assert.NoError(t, pool.AddEvidence(evidence))
	assert.False(t, pool.IsCommitted(evidence))

	// evidence seen and committed:
	pool.MarkEvidenceAsCommitted(height, lastBlockTime, []types.Evidence{evidence})
	assert.True(t, pool.IsCommitted(evidence))
}

func TestEvidencePoolIsNotCommited(t *testing.T) {
	var (
		valAddr           = []byte("validator_address")
		height            = int64(42)
		evidenceBlockTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
		lastBlockTime     = evidenceBlockTime.Add(50 * time.Hour) // default param is 48 hourse
		stateDB           = initializeValidatorState(valAddr, height)
		evidenceDB        = dbm.NewMemDB()
		pool              = NewPool(stateDB, evidenceDB)
	)

	evidence := types.NewMockBadEvidence(height, evidenceBlockTime, 0, valAddr)
	assert.False(t, pool.IsCommitted(evidence))

	// evidence seen but not yet committed:
	assert.Error(t, pool.AddEvidence(evidence))
	assert.False(t, pool.IsCommitted(evidence))

	// evidence seen and committed:
	pool.MarkEvidenceAsCommitted(height, lastBlockTime, []types.Evidence{evidence})
	he := pool.IsCommitted(evidence)
	fmt.Println(he)
	assert.True(t, pool.IsCommitted(evidence))
}
