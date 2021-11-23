package light_test

import (
	"context"
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

// Manually getting light blocks and verifying them.
func ExampleClient() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf, err := rpctest.CreateConfig("ExampleClient_VerifyLightBlockAtHeight")
	if err != nil {
		stdlog.Fatal(err)
	}

	logger := log.TestingLogger()

	// Start a test application
	app := kvstore.NewApplication()

	_, closer, err := rpctest.StartTendermint(ctx, conf, app, rpctest.SuppressStdout)
	if err != nil {
		stdlog.Fatal(err)
	}
	defer func() { _ = closer(ctx) }()

	dbDir, err := os.MkdirTemp("", "light-client-example")
	if err != nil {
		stdlog.Fatal(err)
	}
	defer os.RemoveAll(dbDir)

	chainID := conf.ChainID()

	primary, err := httpp.New(chainID, conf.RPC.ListenAddress)
	if err != nil {
		stdlog.Fatal(err)
	}

	// give Tendermint time to generate some blocks
	time.Sleep(5 * time.Second)

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
		light.Logger(logger),
	)
	if err != nil {
		stdlog.Fatal(err)
	}
	defer func() {
		if err := c.Cleanup(); err != nil {
			stdlog.Fatal(err)
		}
	}()

	// wait for a few more blocks to be produced
	time.Sleep(2 * time.Second)

	// veify the block at height 3
	_, err = c.VerifyLightBlockAtHeight(context.Background(), 3, time.Now())
	if err != nil {
		stdlog.Fatal(err)
	}

	// retrieve light block at height 3
	_, err = c.TrustedLightBlock(3)
	if err != nil {
		stdlog.Fatal(err)
	}

	// update to the latest height
	lb, err := c.Update(ctx, time.Now())
	if err != nil {
		stdlog.Fatal(err)
	}

	logger.Info("verified light block", "light-block", lb)
}
