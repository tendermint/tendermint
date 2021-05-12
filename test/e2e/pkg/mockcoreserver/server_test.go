package mockcoreserver

import (
	"context"
	"fmt"
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
	"github.com/tendermint/tendermint/types"
)

func TestServer(t *testing.T) {
	ctx := context.Background()
	srv := HTTPServer(t, ":9981")
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
			Respond(Body([]byte(tc.e)), JsonContentType())
		resp, err := http.Get(tc.url)
		if err != nil {
			panic(err)
		}
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s\n", data)
	}
	srv.Stop(ctx)
}

func TestPrivValidator(t *testing.T) {
	host := "localhost:19998"
	total := 10
	mockPV := types.NewMockPV()
	conf := PrivValidConfig{
		QuorumType:    btcjson.LLMQType_5_60,
		QuorumHash:    crypto.RandQuorumHash(),
		PrivValidator: mockPV,
	}
	conf.PrivKey, conf.ProTxHash, conf.PubKey = bls12381.CreatePrivLLMQDataDefaultThreshold(total)
	ctx := context.Background()
	srv := HTTPServer(t, host)
	go func() {
		srv.Start()
	}()
	pingRequest(srv)
	client, err := privval.NewDashCoreSignerClient(host, "root", "root", btcjson.LLMQType_5_60, "chain-123456")
	assert.NoError(t, err)
	err = client.Ping()
	assert.NoError(t, err)
	srv.Stop(ctx)
}

func pingRequest(srv *Server) {
	result := []btcjson.GetPeerInfoResult{{}}
	srv.
		On("/").
		Expect(And(BodyShouldBeSame(`{"jsonrpc":"1.0","method":"ping","params":[],"id":1}`))).
		Once().
		Respond(Body([]byte(`"hello"`)), JsonContentType())
	srv.
		On("/").
		Expect(And(BodyShouldBeSame(`{"jsonrpc":"1.0","method":"getpeerinfo","params":[],"id":2}`))).
		Once().
		Respond(Body(mustMarshal(result)), JsonContentType())
}
