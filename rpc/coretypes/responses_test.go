package coretypes

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	pbcrypto "github.com/tendermint/tendermint/proto/tendermint/crypto"
	"github.com/tendermint/tendermint/types"
)

func TestStatusIndexer(t *testing.T) {
	var status *ResultStatus
	assert.False(t, status.TxIndexEnabled())

	status = &ResultStatus{}
	assert.False(t, status.TxIndexEnabled())

	status.NodeInfo = types.NodeInfo{}
	assert.False(t, status.TxIndexEnabled())

	cases := []struct {
		expected bool
		other    types.NodeInfoOther
	}{
		{false, types.NodeInfoOther{}},
		{false, types.NodeInfoOther{TxIndex: "aa"}},
		{false, types.NodeInfoOther{TxIndex: "off"}},
		{true, types.NodeInfoOther{TxIndex: "on"}},
	}

	for _, tc := range cases {
		status.NodeInfo.Other = tc.other
		assert.Equal(t, tc.expected, status.TxIndexEnabled())
	}
}

// A regression test for https://github.com/tendermint/tendermint/issues/8583.
func TestResultBlockResults_regression8583(t *testing.T) {
	const keyData = "0123456789abcdef0123456789abcdef" // 32 bytes
	wantKey := base64.StdEncoding.EncodeToString([]byte(keyData))

	rsp := &ResultBlockResults{
		ValidatorUpdates: []abci.ValidatorUpdate{{
			PubKey: pbcrypto.PublicKey{
				Sum: &pbcrypto.PublicKey_Ed25519{Ed25519: []byte(keyData)},
			},
			Power: 400,
		}},
	}

	// Use compact here so the test data remain legible. The output from the
	// marshaler will have whitespace folded out so we need to do that too for
	// the comparison to be valid.
	var buf bytes.Buffer
	require.NoError(t, json.Compact(&buf, []byte(fmt.Sprintf(`
{
  "height": "0",
  "txs_results": null,
  "total_gas_used": "0",
  "finalize_block_events": null,
  "validator_updates": [
    {
      "pub_key":{"type": "tendermint/PubKeyEd25519", "value": "%s"},
      "power": "400"
    }
  ],
  "consensus_param_updates": null
}`, wantKey))))

	bits, err := json.Marshal(rsp)
	if err != nil {
		t.Fatalf("Encoding block result: %v", err)
	}
	if diff := cmp.Diff(buf.String(), string(bits)); diff != "" {
		t.Errorf("Marshaled result (-want, +got):\n%s", diff)
	}

	back := new(ResultBlockResults)
	if err := json.Unmarshal(bits, back); err != nil {
		t.Fatalf("Unmarshaling: %v", err)
	}
	if diff := cmp.Diff(rsp, back); diff != "" {
		t.Errorf("Unmarshaled result (-want, +got):\n%s", diff)
	}
}
