package light_test

import (
	"context"
	"fmt"
	"io/ioutil"
	stdlog "log"
	"os"
	"time"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/light"
	"github.com/tendermint/tendermint/light/provider"
	httpp "github.com/tendermint/tendermint/light/provider/http"
	dbs "github.com/tendermint/tendermint/light/store/db"
	rpctest "github.com/tendermint/tendermint/rpc/test"
)

// Automatically getting new headers and verifying them.
func ExampleClient_Update() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf := rpctest.CreateConfig()

	// Start a test application
	app := kvstore.NewApplication()
	n, closer, err := rpctest.StartTendermint(ctx, conf, app, rpctest.SuppressStdout)
	if err != nil {
		stdlog.Fatal(err)
	}
	defer closer(ctx)

	if err := n.Start(); err != nil {
		stdlog.Fatal(err)
	}

	// give Tendermint time to generate some blocks
	time.Sleep(5 * time.Second)

	dbDir, err := ioutil.TempDir("", "light-client-example")
	if err != nil {
		stdlog.Fatal(err)
	}
	defer os.RemoveAll(dbDir)

	chainID := conf.ChainID()

	primary, err := httpp.New(chainID, conf.RPC.ListenAddress)
	if err != nil {
		stdlog.Fatal(err)
	}

	block, err := primary.LightBlock(ctx, 2)
	if err != nil {
		stdlog.Fatal(err)
	}

	db, err := dbm.NewGoLevelDB("light-client-db", dbDir)
	if err != nil {
		stdlog.Fatal(err)
	}

	c, err := light.NewClient(
		ctx,
		chainID,
		light.TrustOptions{
			Period: 504 * time.Hour, // 21 days
			Height: 2,
			Hash:   block.Hash(),
		},
		primary,
		[]provider.Provider{primary}, // NOTE: primary should not be used here
		dbs.New(db),
		light.Logger(log.TestingLogger()),
	)
	if err != nil {
		stdlog.Fatal(err)
	}
	defer func() {
		if err := c.Cleanup(); err != nil {
			stdlog.Fatal(err)
		}
	}()

	time.Sleep(2 * time.Second)

	h, err := c.Update(ctx, time.Now())
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

// Manually getting light blocks and verifying them.
func ExampleClient_VerifyLightBlockAtHeight() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf := rpctest.CreateConfig()

	// Start a test application
	app := kvstore.NewApplication()

	n, closer, err := rpctest.StartTendermint(ctx, conf, app, rpctest.SuppressStdout)
	if err != nil {
		stdlog.Fatal(err)
	}
	defer closer(ctx)

	if err := n.Start(); err != nil {
		stdlog.Fatal(err)
	}

	// give Tendermint time to generate some blocks
	time.Sleep(5 * time.Second)

	dbDir, err := ioutil.TempDir("", "light-client-example")
	if err != nil {
		stdlog.Fatal(err)
	}
	defer os.RemoveAll(dbDir)

	chainID := conf.ChainID()

	primary, err := httpp.New(chainID, conf.RPC.ListenAddress)
	if err != nil {
		stdlog.Fatal(err)
	}

	block, err := primary.LightBlock(ctx, 2)
	if err != nil {
		stdlog.Fatal(err)
	}

	db, err := dbm.NewGoLevelDB("light-client-db", dbDir)
	if err != nil {
		stdlog.Fatal(err)
	}

	c, err := light.NewClient(ctx,
		chainID,
		light.TrustOptions{
			Period: 504 * time.Hour, // 21 days
			Height: 2,
			Hash:   block.Hash(),
		},
		primary,
		[]provider.Provider{primary}, // NOTE: primary should not be used here
		dbs.New(db),
		light.Logger(log.TestingLogger()),
	)
	if err != nil {
		stdlog.Fatal(err)
	}
	defer func() {
		if err := c.Cleanup(); err != nil {
			stdlog.Fatal(err)
		}
	}()

	_, err = c.VerifyLightBlockAtHeight(context.Background(), 3, time.Now())
	if err != nil {
		stdlog.Fatal(err)
	}

	h, err := c.TrustedLightBlock(3)
	if err != nil {
		stdlog.Fatal(err)
	}

	fmt.Println("got header", h.Height)
	// Output: got header 3
}
