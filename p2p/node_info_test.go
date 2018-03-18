package p2p_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tmlibs/common"
)

func TestNodeInfoChannelsAreHexEncoded(t *testing.T) {
	chb := "Tendermint"
	ni := &p2p.NodeInfo{
		PubKey:   crypto.GenPrivKeyEd25519().PubKey(),
		Channels: []byte(chb),
		Other:    []string{"test", "foo"},
	}
	gotBlob, err := json.Marshal(ni)
	require.Nil(t, err, "expecting a successful json.Marshal")
	keyBlob, err := json.Marshal(ni.PubKey)
	require.Nil(t, err, "expecting a successful json.Marshal of the key")
	channelsBlob, err := json.Marshal(common.HexBytes(chb))
	require.Nil(t, err, "expecting a successful json.Marshal of the channels")
	wantBlob := []byte(fmt.Sprintf(`{"pub_key":%s,"listen_addr":"","network":"","version":"","channels":%s,"moniker":"","other":["test","foo"]}`, keyBlob, channelsBlob))
	require.Equal(t, wantBlob, gotBlob)

	back := new(p2p.NodeInfo)
	require.Nil(t, json.Unmarshal(gotBlob, back), "roundtrip Unmarshal should not error")
	require.Equal(t, ni, back, "roundtrip Unmarshal should return the same object as the original")
}
