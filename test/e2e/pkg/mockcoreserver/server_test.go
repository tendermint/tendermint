package mockcoreserver

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"testing"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/privval"
)

func TestServer(t *testing.T) {
	ctx := context.Background()
	srv := NewHTTPServer(t, ":9981")
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
		if err != nil {
			panic(err)
		}
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
	srv := NewJRPCServer(t, addr, "/")
	go func() {
		srv.Start()
	}()
	pingRequest(srv)
	client, err := privval.NewDashCoreSignerClient(addr, "root", "root", btcjson.LLMQType_5_60, "chain-123456")
	assert.NoError(t, err)
	err = client.Ping()
	assert.NoError(t, err)
	srv.Stop(ctx)
}

func pingRequest(srv *JRPCServer) {
	result := []btcjson.GetPeerInfoResult{{}}
	srv.
		On("ping").
		Expect(JRPCParamsEmpty()).
		Once().
		Respond(JRPCResult(""), JsonContentType())
	srv.
		On("getpeerinfo").
		Expect(And(JRPCParamsEmpty())).
		Once().
		Respond(JRPCResult(result), JsonContentType())
}
