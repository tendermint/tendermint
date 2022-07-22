package light_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/light"
	"github.com/tendermint/tendermint/light/provider"
	httpp "github.com/tendermint/tendermint/light/provider/http"
	dbs "github.com/tendermint/tendermint/light/store/db"
	rpctest "github.com/tendermint/tendermint/rpc/test"
	"github.com/tendermint/tendermint/types"
)

// NOTE: these are ports of the tests from example_test.go but
// rewritten as more conventional tests.

// Automatically getting new headers and verifying them.
func TestClientIntegration_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)

	logger := log.NewNopLogger()

	// Start a test application
	app := kvstore.NewApplication()
	_, closer, err := rpctest.StartTendermint(ctx, conf, app, rpctest.SuppressStdout)
	require.NoError(t, err)
	defer func() { require.NoError(t, closer(ctx)) }()

	// give Tendermint time to generate some blocks
	time.Sleep(5 * time.Second)

	dbDir := t.TempDir()
	chainID := conf.ChainID()

	primary, err := httpp.New(chainID, conf.RPC.ListenAddress)
	require.NoError(t, err)

	// give Tendermint time to generate some blocks
	block, err := waitForBlock(ctx, primary, 2)
	require.NoError(t, err)

	db, err := dbm.NewGoLevelDB("light-client-db", dbDir)
	require.NoError(t, err)

	c, err := light.NewClient(
		ctx,
		chainID,
		light.TrustOptions{
			Period: 504 * time.Hour, // 21 days
			Height: 2,
			Hash:   block.Hash(),
		},
		primary,
		nil,
		dbs.New(db),
		light.Logger(logger),
	)
	require.NoError(t, err)

	defer func() { require.NoError(t, c.Cleanup()) }()

	// ensure Tendermint is at height 3 or higher
	_, err = waitForBlock(ctx, primary, 3)
	require.NoError(t, err)

	h, err := c.Update(ctx, time.Now())
	require.NoError(t, err)
	require.NotNil(t, h)

	require.True(t, h.Height > 2)
}

// Manually getting light blocks and verifying them.
func TestClientIntegration_VerifyLightBlockAtHeight(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)

	logger := log.NewNopLogger()

	// Start a test application
	app := kvstore.NewApplication()

	_, closer, err := rpctest.StartTendermint(ctx, conf, app, rpctest.SuppressStdout)
	require.NoError(t, err)
	defer func() { require.NoError(t, closer(ctx)) }()

	dbDir := t.TempDir()
	chainID := conf.ChainID()

	primary, err := httpp.New(chainID, conf.RPC.ListenAddress)
	require.NoError(t, err)

	// give Tendermint time to generate some blocks
	block, err := waitForBlock(ctx, primary, 2)
	require.NoError(t, err)

	db, err := dbm.NewGoLevelDB("light-client-db", dbDir)
	require.NoError(t, err)

	c, err := light.NewClient(ctx,
		chainID,
		light.TrustOptions{
			Period: 504 * time.Hour, // 21 days
			Height: 2,
			Hash:   block.Hash(),
		},
		primary,
		nil,
		dbs.New(db),
		light.Logger(logger),
	)
	require.NoError(t, err)

	defer func() { require.NoError(t, c.Cleanup()) }()

	// ensure Tendermint is at height 3 or higher
	_, err = waitForBlock(ctx, primary, 3)
	require.NoError(t, err)

	_, err = c.VerifyLightBlockAtHeight(ctx, 3, time.Now())
	require.NoError(t, err)

	h, err := c.TrustedLightBlock(3)
	require.NoError(t, err)

	require.EqualValues(t, 3, h.Height)
}

func waitForBlock(ctx context.Context, p provider.Provider, height int64) (*types.LightBlock, error) {
	for {
		block, err := p.LightBlock(ctx, height)
		switch err {
		case nil:
			return block, nil
		// node isn't running yet, wait 1 second and repeat
		case provider.ErrNoResponse, provider.ErrHeightTooHigh:
			timer := time.NewTimer(1 * time.Second)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-timer.C:
			}
		default:
			return nil, err
		}
	}
}

func TestClientStatusRPC(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)

	// Start a test application
	app := kvstore.NewApplication()

	_, closer, err := rpctest.StartTendermint(ctx, conf, app, rpctest.SuppressStdout)
	require.NoError(t, err)
	defer func() { require.NoError(t, closer(ctx)) }()

	dbDir := t.TempDir()
	chainID := conf.ChainID()

	primary, err := httpp.New(chainID, conf.RPC.ListenAddress)
	require.NoError(t, err)

	// give Tendermint time to generate some blocks
	block, err := waitForBlock(ctx, primary, 2)
	require.NoError(t, err)

	db, err := dbm.NewGoLevelDB("light-client-db", dbDir)
	require.NoError(t, err)

	// In order to not create a full testnet we create the light client with no witnesses
	// and only verify the primary IP address.
	witnesses := []provider.Provider{}

	c, err := light.NewClient(ctx,
		chainID,
		light.TrustOptions{
			Period: 504 * time.Hour, // 21 days
			Height: 2,
			Hash:   block.Hash(),
		},
		primary,
		witnesses,
		dbs.New(db),
		light.Logger(log.NewNopLogger()),
	)
	require.NoError(t, err)

	defer func() { require.NoError(t, c.Cleanup()) }()

	lightStatus := c.Status(ctx)

	// Verify primary IP
	require.True(t, lightStatus.PrimaryID == primary.ID())

	// Verify that number of peers is equal to number of witnesses  (+ 1 if the primary is not a witness)
	require.Equal(t, len(witnesses)+1*primaryNotInWitnessList(witnesses, primary), lightStatus.NumPeers)

	// Verify that the last trusted hash returned matches the stored hash of the trusted
	// block at the last trusted height.
	blockAtTrustedHeight, err := c.TrustedLightBlock(lightStatus.LastTrustedHeight)
	require.NoError(t, err)

	require.EqualValues(t, lightStatus.LastTrustedHash, blockAtTrustedHeight.Hash())

}

// If the primary is not in the witness list, we will return 1
// Otherwise, return 0
func primaryNotInWitnessList(witnesses []provider.Provider, primary provider.Provider) int {
	for _, el := range witnesses {
		if el == primary {
			return 0
		}
	}
	return 1
}
