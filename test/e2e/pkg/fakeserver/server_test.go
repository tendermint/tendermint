package fakeserver

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"testing"
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
