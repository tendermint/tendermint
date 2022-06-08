package light_test

import (
	"context"
	"testing"
	"time"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/privval"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	dashcore "github.com/tendermint/tendermint/dash/core"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/light"
	httpp "github.com/tendermint/tendermint/light/provider/http"
	dbs "github.com/tendermint/tendermint/light/store/db"
	rpctest "github.com/tendermint/tendermint/rpc/test"
)

// Manually getting light blocks and verifying them.
func TestExampleClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf, err := rpctest.CreateConfig(t, "ExampleClient_VerifyLightBlockAtHeight")
	if err != nil {
		t.Fatal(err)
	}

	logger, err := log.NewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo)
	if err != nil {
		t.Fatal(err)
	}

	// Start a test application
	app := kvstore.NewApplication()

	_, closer, err := rpctest.StartTendermint(ctx, conf, app, rpctest.SuppressStdout)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closer(ctx) }()

	dbDir := t.TempDir()
	chainID := conf.ChainID()

	primary, err := httpp.New(chainID, conf.RPC.ListenAddress)
	if err != nil {
		t.Fatal(err)
	}

	// give Tendermint time to generate some blocks
	time.Sleep(5 * time.Second)

	_, err = primary.LightBlock(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}

	db, err := dbm.NewGoLevelDB("light-client-db", dbDir)
	if err != nil {
		t.Fatal(err)
	}

	pv, err := privval.LoadOrGenFilePV(conf.PrivValidator.KeyFile(), conf.PrivValidator.StateFile())
	require.NoError(t, err)

	c, err := light.NewClient(
		ctx,
		chainID,
		primary,
		nil,
		dbs.New(db),
		dashcore.NewMockClient(chainID, btcjson.LLMQType_5_60, pv, false),
		light.Logger(log.NewTestingLogger(t)),
	)

	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := c.Cleanup(); err != nil {
			t.Fatal(err)
		}
	}()

	// wait for a few more blocks to be produced
	time.Sleep(2 * time.Second)

	// verify the block at height 3
	_, err = c.VerifyLightBlockAtHeight(ctx, 3, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	// retrieve light block at height 3
	_, err = c.TrustedLightBlock(3)
	if err != nil {
		t.Fatal(err)
	}

	// update to the latest height
	lb, err := c.Update(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	logger.Info("verified light block", "light-block", lb)
}
