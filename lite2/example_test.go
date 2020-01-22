package lite

import (
	"fmt"
	"io/ioutil"
	stdlog "log"
	"os"
	"testing"
	"time"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/libs/log"
	httpp "github.com/tendermint/tendermint/lite2/provider/http"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
	rpctest "github.com/tendermint/tendermint/rpc/test"
)

// Automatically getting new headers and verifying them.
func TestExample_Client_AutoUpdate(t *testing.T) {
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

	provider, err := httpp.New(chainID, config.RPC.ListenAddress)
	if err != nil {
		stdlog.Fatal(err)
	}

	header, err := provider.SignedHeader(2)
	if err != nil {
		stdlog.Fatal(err)
	}

	db, err := dbm.NewGoLevelDB("lite-client-db", dbDir)
	if err != nil {
		stdlog.Fatal(err)
	}

	c, err := NewClient(
		chainID,
		TrustOptions{
			Period: 504 * time.Hour, // 21 days
			Height: 2,
			Hash:   header.Hash(),
		},
		provider,
		dbs.New(db, chainID),
		UpdatePeriod(1*time.Second),
		Logger(log.TestingLogger()),
	)
	if err != nil {
		stdlog.Fatal(err)
	}
	defer c.Stop()

	time.Sleep(2 * time.Second)

	h, err := c.TrustedHeader(3, time.Now())
	if err != nil {
		stdlog.Fatal(err)
	}

	fmt.Println("got header", h.Height)
	// Output: got header 3
}

// Manually getting headers and verifying them.
func TestExample_Client_ManualUpdate(t *testing.T) {
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

	provider, err := httpp.New(chainID, config.RPC.ListenAddress)
	if err != nil {
		stdlog.Fatal(err)
	}

	header, err := provider.SignedHeader(2)
	if err != nil {
		stdlog.Fatal(err)
	}

	db, err := dbm.NewGoLevelDB("lite-client-db", dbDir)
	if err != nil {
		stdlog.Fatal(err)
	}

	c, err := NewClient(
		chainID,
		TrustOptions{
			Period: 504 * time.Hour, // 21 days
			Height: 2,
			Hash:   header.Hash(),
		},
		provider,
		dbs.New(db, chainID),
		UpdatePeriod(0),
		Logger(log.TestingLogger()),
	)
	if err != nil {
		stdlog.Fatal(err)
	}
	defer c.Stop()

	_, err = c.VerifyHeaderAtHeight(3, time.Now())
	if err != nil {
		stdlog.Fatal(err)
	}

	h, err := c.TrustedHeader(3, time.Now())
	if err != nil {
		stdlog.Fatal(err)
	}

	fmt.Println("got header", h.Height)
	// Output: got header 3
}

func TestMain(m *testing.M) {
	// start a tendermint node (and kvstore) in the background to test against
	app := kvstore.NewApplication()
	node := rpctest.StartTendermint(app)

	code := m.Run()

	// and shut down proper at the end
	rpctest.StopTendermint(node)
	os.Exit(code)
}
