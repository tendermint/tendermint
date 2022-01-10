package light_test

import (
	"context"
	"os"
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
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf, err := rpctest.CreateConfig(t.Name())
	require.NoError(t, err)

	// Start a test application
	app := kvstore.NewApplication()
	_, closer, err := rpctest.StartTendermint(ctx, conf, app, rpctest.SuppressStdout)
	require.NoError(t, err)
	defer func() { require.NoError(t, closer(ctx)) }()

	// give Tendermint time to generate some blocks
	time.Sleep(5 * time.Second)

	dbDir, err := os.MkdirTemp("", "light-client-test-update-example")
	require.NoError(t, err)
	defer os.RemoveAll(dbDir)

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
		[]provider.Provider{primary}, // NOTE: primary should not be used here
		dbs.New(db),
		light.Logger(log.TestingLogger()),
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
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf, err := rpctest.CreateConfig(t.Name())
	require.NoError(t, err)

	// Start a test application
	app := kvstore.NewApplication()

	_, closer, err := rpctest.StartTendermint(ctx, conf, app, rpctest.SuppressStdout)
	require.NoError(t, err)
	defer func() { require.NoError(t, closer(ctx)) }()

	dbDir, err := os.MkdirTemp("", "light-client-test-verify-example")
	require.NoError(t, err)
	defer os.RemoveAll(dbDir)

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
		[]provider.Provider{primary}, // NOTE: primary should not be used here
		dbs.New(db),
		light.Logger(log.TestingLogger()),
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf, err := rpctest.CreateConfig(t.Name())
	require.NoError(t, err)

	// Start a test application
	app := kvstore.NewApplication()

	_, closer, err := rpctest.StartTendermint(ctx, conf, app, rpctest.SuppressStdout)
	require.NoError(t, err)
	defer func() { require.NoError(t, closer(ctx)) }()

	dbDir, err := os.MkdirTemp("", "light-client-test-status-example")
	require.NoError(t, err)
	defer os.RemoveAll(dbDir)

	chainID := conf.ChainID()

	primary, err := httpp.New(chainID, conf.RPC.ListenAddress)
	require.NoError(t, err)

	witnesses := []provider.Provider{primary}
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
		witnesses,
		dbs.New(db),
		light.Logger(log.TestingLogger()),
	)
	require.NoError(t, err)

	defer func() { require.NoError(t, c.Cleanup()) }()

	lightStatus, err := c.Status(ctx)

	// Verify primary IP
	require.True(t, lightStatus.Primary == primary.String())

	// Verify IPs of witnesses
	// ToDo - Add test with multiple witnesses
	require.ElementsMatch(t, mapProviderArrayToIP(witnesses), lightStatus.Witnesses)

	// Verify that the last trusted height of a block matches the last trusted height stored in the store
	require.True(t, lightStatus.LastTrustedBlockHeight == lightStatus.LastTrustedHeight)

	// Verify that the last trusted hash returned matches the stored hash of the trusted block at the last trusted height
	blockAtTrustedHeight, err := c.TrustedLightBlock(lightStatus.LastTrustedBlockHeight)
	require.NoError(t, err)

	require.EqualValues(t, lightStatus.LastTrustedHash, blockAtTrustedHeight.Hash())

	// Verify that number of peers is equal to number of witnesses  (+ 1 if the primary is not a witness)
	require.Equal(t, 1, lightStatus.NumPeers)

}

func mapProviderArrayToIP(el []provider.Provider) []string {
	tmpArray := make([]string, len(el))
	for i, v := range el {
		tmpArray[i] = v.String()
	}
	return tmpArray
}
