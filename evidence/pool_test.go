package evidence

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tmlibs/db"
)

var mockState = sm.State{}

func TestEvidencePool(t *testing.T) {
	assert := assert.New(t)

	params := types.EvidenceParams{}
	store := NewEvidenceStore(dbm.NewMemDB())
	pool := NewEvidencePool(params, store, mockState)

	goodEvidence := newMockGoodEvidence(5, 1, []byte("val1"))
	badEvidence := MockBadEvidence{goodEvidence}

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
