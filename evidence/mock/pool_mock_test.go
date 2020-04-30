package mock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/types"
)

func TestMockEvidencePool(t *testing.T) {
	pool := NewDefaultEvidencePool()
	ev := pool.AddMockEvidence(1, []byte("val_address"))
	assert.True(t, pool.IsPending(ev))
	assert.Equal(t, []types.Evidence{ev}, pool.PendingEvidence(-1))
	pool.CommitEvidence(ev)
	assert.True(t, pool.IsCommitted(ev))
	ev2 := pool.AddMockEvidence(2, []byte("val_address"))
	pool.BlockTime = time.Now().Add(time.Millisecond)
	pool.BlockHeight = 4
	pool.RemoveExpiredEvidence()
	assert.False(t, pool.IsPending(ev2))
	ev3 := pool.AddMockEvidence(3, []byte("val_address"))
	pool.RemovePendingEvidence(ev3)
	assert.Equal(t, 0, len(pool.PendingEvidenceList))
	pubKey, _ := types.NewMockPV().GetPubKey()
	polc := types.NewMockPOLC(1, time.Now(), pubKey)
	_ = pool.AddPOLC(polc)
	polc2, err := pool.RetrievePOLC(1, 1)
	assert.NoError(t, err)
	assert.True(t, polc.Equal(polc2))
	_, err = pool.RetrievePOLC(2, 1)
	assert.Error(t, err)
}
