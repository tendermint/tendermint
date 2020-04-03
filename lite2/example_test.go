package lite_test

import (
	"fmt"
	"io/ioutil"
	stdlog "log"
	"os"
	"testing"
	"time"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	lite "github.com/tendermint/tendermint/lite2"
	"github.com/tendermint/tendermint/lite2/provider"
	httpp "github.com/tendermint/tendermint/lite2/provider/http"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
	rpctest "github.com/tendermint/tendermint/rpc/test"
)

// Automatically getting new headers and verifying them.
func ExampleClient_Update() {
	// give Tendermint time to generate some blocks
	time.Sleep(5 * time.Second)

	dbDir, err := ioutil.TempDir("", "lite-client-example")
	if err != nil {
		stdlog.Fatal(err)
	}
	defer os.RemoveAll(dbDir)

	var (
		config  = rpctest.GetConfig()
		chainID = config.ChainID()
	)

	primary, err := httpp.New(chainID, config.RPC.ListenAddress)
	if err != nil {
		stdlog.Fatal(err)
	}

	header, err := primary.SignedHeader(2)
	if err != nil {
		stdlog.Fatal(err)
	}

	db, err := dbm.NewGoLevelDB("lite-client-db", dbDir)
	if err != nil {
		stdlog.Fatal(err)
	}

	c, err := lite.NewClient(
		chainID,
		lite.TrustOptions{
			Period: 504 * time.Hour, // 21 days
			Height: 2,
			Hash:   header.Hash(),
		},
		primary,
		[]provider.Provider{primary}, // NOTE: primary should not be used here
		dbs.New(db, chainID),
		// Logger(log.TestingLogger()),
	)
	if err != nil {
		stdlog.Fatal(err)
	}
	defer func() {
		c.Cleanup()
	}()

	time.Sleep(2 * time.Second)

	// XXX: 30 * time.Minute clock drift is needed because a) Tendermint strips
	// monotonic component (see types/time/time.go) b) single instance is being
	// run.
	// https://github.com/tendermint/tendermint/issues/4489
	h, err := c.Update(time.Now().Add(30 * time.Minute))
	if err != nil {
		stdlog.Fatal(err)
	}

	if h != nil && h.Height > 2 {
		fmt.Println("successful update")
	} else {
		fmt.Println("update failed")
	}
	// Output: successful update
}

// Manually getting headers and verifying them.
func ExampleClient_VerifyHeaderAtHeight() {
	// give Tendermint time to generate some blocks
	time.Sleep(5 * time.Second)

	dbDir, err := ioutil.TempDir("", "lite-client-example")
	if err != nil {
		stdlog.Fatal(err)
	}
	defer os.RemoveAll(dbDir)

	var (
		config  = rpctest.GetConfig()
		chainID = config.ChainID()
	)

	primary, err := httpp.New(chainID, config.RPC.ListenAddress)
	if err != nil {
		stdlog.Fatal(err)
	}

	header, err := primary.SignedHeader(2)
	if err != nil {
		stdlog.Fatal(err)
	}

	db, err := dbm.NewGoLevelDB("lite-client-db", dbDir)
	if err != nil {
		stdlog.Fatal(err)
	}

	c, err := lite.NewClient(
		chainID,
		lite.TrustOptions{
			Period: 504 * time.Hour, // 21 days
			Height: 2,
			Hash:   header.Hash(),
		},
		primary,
		[]provider.Provider{primary}, // NOTE: primary should not be used here
		dbs.New(db, chainID),
		// Logger(log.TestingLogger()),
	)
	if err != nil {
		stdlog.Fatal(err)
	}
	defer func() {
		c.Cleanup()
	}()

	_, err = c.VerifyHeaderAtHeight(3, time.Now())
	if err != nil {
		stdlog.Fatal(err)
	}

	h, err := c.TrustedHeader(3)
	if err != nil {
		stdlog.Fatal(err)
	}

	fmt.Println("got header", h.Height)
	// Output: got header 3
}

func TestMain(m *testing.M) {
	// start a tendermint node (and kvstore) in the background to test against
	app := kvstore.NewApplication()
	node := rpctest.StartTendermint(app, rpctest.SuppressStdout)

	code := m.Run()

	// and shut down proper at the end
	rpctest.StopTendermint(node)
	os.Exit(code)
}
