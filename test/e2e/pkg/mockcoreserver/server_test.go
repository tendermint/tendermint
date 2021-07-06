package mockcoreserver

import (
	"context"
	"encoding/hex"
	dashcore "github.com/tendermint/tendermint/dashcore/rpc"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"testing"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/privval"
)

func TestServer(t *testing.T) {
	ctx := context.Background()
	srv := NewHTTPServer(":9981")
	go func() {
		srv.Start()
	}()
	testCases := []struct {
		url   string
		e     string
		query url.Values
	}{
		{
			url:   "http://localhost:9981/test",
			e:     "dash is the best coin",
			query: url.Values{},
		},
		{
			url: "http://localhost:9981/test?q1=100&q2=bc",
			e:   "dash is the best ever coin",
			query: url.Values{
				"q1": []string{"100"},
				"q2": []string{"bc"},
			},
		},
	}
	for _, tc := range testCases {
		srv.
			On("/test").
			Expect(func(req *http.Request) error {
				log.Println(req.URL.String())
				return nil
			}).
			Expect(And(BodyShouldBeEmpty(), QueryShouldHave(tc.query))).
			Once().
			Respond(JsonBody(tc.e), JsonContentType())
		resp, err := http.Get(tc.url)
		assert.NoError(t, err)
		data, err := ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()
		assert.NoError(t, err)
		s := ""
		mustUnmarshal(data, &s)
		assert.Equal(t, tc.e, s)
	}
	srv.Stop(ctx)
}

func TestDashCoreSignerPingMethod(t *testing.T) {
	addr := "localhost:19998"
	ctx := context.Background()
	srv := NewJRPCServer(addr, "/")
	go func() {
		srv.Start()
	}()
	cs := &StaticCoreServer{}
	srv = WithMethods(
		srv,
		WithPingMethod(cs, 1),
	)
	dashCoreRpcClient, err := dashcore.NewDashCoreRpcClient(addr, "root", "root")
	assert.NoError(t, err)
	client, err := privval.NewDashCoreSignerClient(dashCoreRpcClient, btcjson.LLMQType_5_60)
	assert.NoError(t, err)
	err = client.Ping()
	assert.NoError(t, err)
	srv.Stop(ctx)
}

func TestGetPubKey(t *testing.T) {
	addr := "localhost:19998"
	ctx := context.Background()
	srv := NewJRPCServer(addr, "/")
	go func() {
		srv.Start()
	}()
	proTxHash := "6c91363d97b286e921afb5cf7672c88a2f1614d36d32058c34bef8b44e026007"
	cs := &StaticCoreServer{
		QuorumInfoResult: btcjson.QuorumInfoResult{
			Height:     1010,
			Type:       "llmq_50_60",
			QuorumHash: "000004bfc56646880bfeb80a0b89ad955e557ead7b0f09bcc61e56c8473eaea9",
			MinedBlock: "",
			Members: []btcjson.QuorumMember{
				{
					ProTxHash:      "6c91363d97b286e921afb5cf7672c88a2f1614d36d32058c34bef8b44e026007",
					PubKeyOperator: "81749ba8363e5c03e9d6318b0491e38305cf59d9d57cea2295a86ecfa696622571f266c28bacc78666e8b9b0fb2b3121",
					Valid:          true,
					PubKeyShare:    "83349ba8363e5c03e9d6318b0491e38305cf59d9d57cea2295a86ecfa696622571f266c28bacc78666e8b9b0fb2b3123",
				},
			},
			QuorumPublicKey: "0644ff153b9b92c6a59e2adf4ef0b9836f7f6af05fe432ffdcb69bc9e300a2a70af4a8d9fc61323f6b81074d740033d2",
			SecretKeyShare:  "3da0d8f532309660f7f44aa0ed42c1569773b39c70f5771ce5604be77e50759e",
		},
		MasternodeStatusResult: btcjson.MasternodeStatusResult{
			ProTxHash: proTxHash,
		},
		GetNetworkInfoResult: btcjson.GetNetworkInfoResult{},
	}
	srv = WithMethods(
		srv,
		WithQuorumInfoMethod(cs, Endless),
		WithMasternodeMethod(cs, Endless),
		WithGetNetworkInfoMethod(cs, Endless),
	)

	dashCoreRpcClient, err := dashcore.NewDashCoreRpcClient(addr, "root", "root")
	assert.NoError(t, err)
	client, err := privval.NewDashCoreSignerClient(dashCoreRpcClient, btcjson.LLMQType_5_60)
	assert.NoError(t, err)
	quorumHash := crypto.RandQuorumHash()
	pubKey, err := client.GetPubKey(quorumHash)
	assert.NoError(t, err)
	b, _ := hex.DecodeString("83349BA8363E5C03E9D6318B0491E38305CF59D9D57CEA2295A86ECFA696622571F266C28BACC78666E8B9B0FB2B3123")
	assert.True(t, pubKey.Equals(bls12381.PubKey(b)))
	srv.Stop(ctx)
}
