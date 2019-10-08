package lite

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	mockp "github.com/tendermint/tendermint/lite2/provider/mock"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
	"github.com/tendermint/tendermint/types"
)

// TODO: consolidate 1,2,3,4
func TestClientSequentialVerification1(t *testing.T) {
	const (
		chainID = "TestClientSequentialVerification1"
	)

	var (
		keys = genPrivKeys(4)
		// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
		vals     = keys.ToValidators(20, 10)
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		header   = keys.GenSignedHeader(chainID, 1, bTime, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
	)

	c, err := NewClient(
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
				2: keys.GenSignedHeader(chainID, 2, bTime.Add(1*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
				// last header (3/3 signed)
				3: keys.GenSignedHeader(chainID, 3, bTime.Add(2*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: vals,
				4: vals,
			},
		),
		dbs.New(dbm.NewMemDB(), ""),
		SequentialVerification(),
	)
	require.NoError(t, err)

	err = c.VerifyHeaderAtHeight(3, bTime.Add(3*time.Hour))
	assert.NoError(t, err)
}

func TestClientSequentialVerification2(t *testing.T) {
	const (
		chainID = "TestClientSequentialVerification2"
	)

	var (
		keys = genPrivKeys(4)
		// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
		vals     = keys.ToValidators(20, 10)
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		header   = keys.GenSignedHeader(chainID, 1, bTime, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
	)

	_, err := NewClient(
		chainID,
		TrustOptions{
			Period: 4 * time.Hour,
			Height: 1,
			Hash:   header.Hash(),
		},
		mockp.New(
			chainID,
			map[int64]*types.SignedHeader{
				// different header
				1: keys.GenSignedHeader(chainID, 1, bTime.Add(1*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			},
			map[int64]*types.ValidatorSet{
				1: vals,
			},
		),
		dbs.New(dbm.NewMemDB(), ""),
		SequentialVerification(),
	)
	assert.Error(t, err)
	// TODO assert errors type or content?
}

func TestClientSequentialVerification3(t *testing.T) {
	const (
		chainID = "TestClientSequentialVerification3"
	)

	var (
		keys = genPrivKeys(4)
		// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
		vals     = keys.ToValidators(20, 10)
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		header   = keys.GenSignedHeader(chainID, 1, bTime, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
	)

	c, err := NewClient(
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
				// interim header (1/3 signed)
				2: keys.GenSignedHeader(chainID, 2, bTime.Add(1*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), len(keys)-1, len(keys)),
				// last header (3/3 signed)
				3: keys.GenSignedHeader(chainID, 3, bTime.Add(2*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: vals,
				4: vals,
			},
		),
		dbs.New(dbm.NewMemDB(), ""),
		SequentialVerification(),
	)
	require.NoError(t, err)

	err = c.VerifyHeaderAtHeight(3, bTime.Add(3*time.Hour))
	assert.Error(t, err)
	// TODO assert errors type or content?
}

func TestClientSequentialVerification4(t *testing.T) {
	const (
		chainID = "TestClientSequentialVerification4"
	)

	var (
		keys = genPrivKeys(4)
		// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
		vals     = keys.ToValidators(20, 10)
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		header   = keys.GenSignedHeader(chainID, 1, bTime, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
	)

	c, err := NewClient(
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
				2: keys.GenSignedHeader(chainID, 2, bTime.Add(1*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
				// last header (1/3 signed)
				3: keys.GenSignedHeader(chainID, 3, bTime.Add(2*time.Hour), nil, vals, vals,
					[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), len(keys)-1, len(keys)),
			},
			map[int64]*types.ValidatorSet{
				1: vals,
				2: vals,
				3: vals,
				4: vals,
			},
		),
		dbs.New(dbm.NewMemDB(), ""),
		SequentialVerification(),
	)
	require.NoError(t, err)

	err = c.VerifyHeaderAtHeight(3, bTime.Add(3*time.Hour))
	assert.Error(t, err)
	// TODO assert errors type or content?
}

func TestClientBisectingVerification(t *testing.T) {
}
