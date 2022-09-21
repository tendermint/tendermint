package app

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	tmtypes "github.com/tendermint/tendermint/proto/tendermint/types"
)

func newApp(t *testing.T) *Application {
	dir := filepath.Join(os.TempDir(), t.Name())
	cfg := kvstore.DefaultConfig(dir)

	app, err := NewApplication(cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		app.Close()
		os.RemoveAll(dir)
	})

	return app
}

func TestPrepareFinalize(t *testing.T) {
	ctx := context.TODO()

	height := int64(1)
	app := newApp(t)
	now := time.Now()

	const (
		key   = "my-tx"
		value = "abc"
	)

	reqPrep := &abci.RequestPrepareProposal{
		MaxTxBytes: 1024000,
		Height:     height,
		Txs: [][]byte{
			[]byte(key + "=" + value),
		},
		Time: now,
	}
	respPrep, err := app.PrepareProposal(ctx, reqPrep)
	require.NoError(t, err)

	txs := make([][]byte, 0, len(respPrep.TxRecords))
	bz := []byte{}
	for _, tx := range respPrep.TxRecords {
		if tx.Action != abci.TxRecord_REMOVED {
			txs = append(txs, tx.Tx)
			bz = append(bz, tx.Tx...)
		}
	}

	bz = append(bz, []byte(strconv.FormatInt(height, 10))...)
	hash := crypto.Checksum(bz)
	reqFinalized := abci.RequestFinalizeBlock{
		Txs:     txs,
		AppHash: respPrep.AppHash,
		Height:  height,
		Hash:    hash,
		Time:    now,
	}

	_, err = app.FinalizeBlock(ctx, &reqFinalized)
	require.NoError(t, err)

	respQuery, err := app.Query(ctx, &abci.RequestQuery{
		Data: []byte("my-tx"),
	})
	require.NoError(t, err)
	assert.Equal(t, code.CodeTypeOK, respQuery.Code, respQuery.Log)
}

func TestPrepareProposal(t *testing.T) {
	testCases := []struct {
		request abci.RequestPrepareProposal
	}{
		{
			request: abci.RequestPrepareProposal{
				Height:     1,
				Time:       time.Now(),
				MaxTxBytes: 1024000,
				Txs: [][]byte{b64(`bG9hZC0zMT1Fd3FuaUNTVlNwWXdBZFI0MGZXbVZJam1qWjR5M0JMdFY3cjVWRUVjaXE2bjlzYVVaY2gzdlZYY3g1MU9STXFpRE1RSVZxSjRCdDlDbzB3NEEySTloT0pvR
nRWcmFmaGdQMk1SOEw3djRscHFYdnQ5dTFZcXFpa0dMOTRCbUZBZk14RVBlVUE0ckxMN1ExSW5KVHJQVUM5akFwUjRDTmNGc1dpSGREMXlWaEFUdjl0c0JjTFhtS043c3BXWmEx
c092bXcyMXB6YkVubm9qYzZoM1ZTMWlyZFkzdnptOGhGVFlSTGhTZFI4SFBYd0U1U2xaYUNHVkF6ODBNbVFsazA3WmF2dTNrZmVibm1HQ1J2cUZRYk9FaDF4UVQwWWw5VW1yenh
0V3YyYjJXbDV5TXVlajBKRzdXczFFV2dUQmlkSEZsM1RhOVBpREZ0WHp0cGdWeGlyMjc4d080S1M2Nmd0M1EweGdRTVJiVTg2UURTNE5pbVNGOVNrNmNkNnJjV3VQWldmMEU5dm
4yaWF2aXI5NEFNNjlHRER4ZW51cndwMWNweHUyUzRReEd1dFZObm1WdVNtUzBVSjUzS1haNDhLVWx3TE9GRjVDaURXTW9WSUo4MGdxSFhzdEpCTWRUVWhobUkzZEdIbDA2bnFtV
XBSZUFOcEtudzBXdEk1dHZMcHJxbWhpTkxocExaQmpKYUdWNDVtSENZOWxVbVFWT0hJeUhvbFVTbWw3RWtyQXpHQjdjNG1QNnRHcUJucHNkcDZNV1B6ZjlvbVV3aDlmZnlwUERj
V1ZMbHhjR0xmSzE1czlhVER1ZkVuQlFlSWFUb0hDVFA2Ykd1dXc4eHE0Zlpjcms1WXlCVDNDODVKU2ptek1ESTFOSkd6cXVtWGJIVzBzVnloRjZBY2FkSmJDMVRyUks1dlRXdGR
CRTh3SnZkYXlCMXJIckh4cEQ1MVdJb3AwUUlPTElkNWZxRm5UYlV6RG9WS0JqbDQ3WVo3S2NKTlJVZGRxenQwa1NrMU01OU5HTWozekQxcHptRmxUQWFrZmJtTjdMR1I2cklTWF
JlV2MyNEVGNWhEd2p2WVpOZXU3aUV1VHEzTUt4ZUN0Q3pFelFYU1p2T1BBVDFLeXZ2WlVVV1hpaUl1c09zRmFEeUFkODZMdVZNd0tLbEVwRVBxRFI0UXBlRGVvYnNYbnRDOW1QW
jlUczRFT1lEWnkxVjNuWlRpOUFWOUkxSEF2RVo4b1ZVTW9OcUtIVUxpSFNPc2lNZFdJTFRoQVVmZU1yTGhVN25DekJIem81R3Jmb2dGZTdYQ2hib2puV1p1VXFOMGhxSWFHZEJl
Ym1saWRueGJDSmpZdXBUTkNNdFpMcUdC`)},
				LocalLastCommit: abci.ExtendedCommitInfo{
					ThresholdVoteExtensions: []*tmtypes.VoteExtension{
						{
							Type:      tmtypes.VoteExtensionType_THRESHOLD_RECOVER,
							Extension: b64("dGhyZXNob2xkLTI="),
							Signature: b64(`iLtq+qk8Rg5Y7bT0lMS+d3BC3RaxBstC7PnFOFESzu/F0il9zo+eR7MWlwsHlctMGR6sG0AgdJ/FdIySA3ueiwUc
							7gG50RjMBgjbb/4ogADm/ON3N9aLFwAABzD+523p`),
						},
					},
				},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			ctx := context.TODO()

			app := newApp(t)

			respPrep, err := app.PrepareProposal(ctx, &tc.request)
			require.NoError(t, err)
			assert.NotEmpty(t, respPrep.AppHash)
		})
	}

}
func b64(s string) []byte {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\t", "")
	bz, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic("cannot base64-decode '" + s + "': " + err.Error())
	}
	return bz
}
