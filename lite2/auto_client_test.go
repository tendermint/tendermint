package lite

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/libs/log"
	mockp "github.com/tendermint/tendermint/lite2/provider/mock"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
	"github.com/tendermint/tendermint/types"
)

func TestAutoClient(t *testing.T) {
	const (
		chainID = "TestAutoClient"
	)

	var (
		keys = genPrivKeys(4)
		// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
		vals   = keys.ToValidators(20, 10)
		bTime  = time.Now().Add(-1 * time.Hour)
		header = keys.GenSignedHeader(chainID, 1, bTime, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
	)

	base, err := NewClient(
		chainID,
		TrustOptions{
			Period: 4 * time.Hour,
			Height: 1,
			Hash:   header.Hash(),
		},
		mockp.New(
			chainID,
			map[int64]*types.SignedHeader{
				// trusted header
				1: header,
				// interim header (3/3 signed)
				2: keys.GenSignedHeader(chainID, 2, bTime.Add(30*time.Minute), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
				// last header (3/3 signed)
				3: keys.GenSignedHeader(chainID, 3, bTime.Add(1*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: vals,
				4: vals,
			},
		),
		dbs.New(dbm.NewMemDB(), chainID),
	)
	require.NoError(t, err)
	base.SetLogger(log.TestingLogger())

	c := NewAutoClient(base, 1*time.Second)
	defer c.Stop()

	for i := 2; i <= 3; i++ {
		select {
		case h := <-c.TrustedHeaders():
			assert.EqualValues(t, i, h.Height)
		case err := <-c.Errs():
			require.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("no headers/errors received in 2 sec")
		}
	}
}
