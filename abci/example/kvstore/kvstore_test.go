package kvstore

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/require"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/code"
	abciserver "github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
)

const (
	testKey   = "abc"
	testValue = "def"
)

func testKVStore(ctx context.Context, t *testing.T, app types.Application, tx []byte, key, value string) {
	req := &types.RequestFinalizeBlock{Txs: [][]byte{tx}}
	ar, err := app.FinalizeBlock(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 1, len(ar.TxResults))
	require.False(t, ar.TxResults[0].IsErr())
	// repeating tx doesn't raise error
	ar, err = app.FinalizeBlock(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 1, len(ar.TxResults))
	require.False(t, ar.TxResults[0].IsErr())
	// commit
	_, err = app.Commit(ctx)
	require.NoError(t, err)

	info, err := app.Info(ctx, &types.RequestInfo{})
	require.NoError(t, err)
	require.NotZero(t, info.LastBlockHeight)

	// make sure query is fine
	resQuery, err := app.Query(ctx, &types.RequestQuery{
		Path: "/store",
		Data: []byte(key),
	})

	require.NoError(t, err)
	require.Equal(t, code.CodeTypeOK, resQuery.Code)
	require.Equal(t, key, string(resQuery.Key))
	require.Equal(t, value, string(resQuery.Value))
	require.EqualValues(t, info.LastBlockHeight, resQuery.Height)

	// make sure proof is fine
	resQuery, err = app.Query(ctx, &types.RequestQuery{
		Path:  "/store",
		Data:  []byte(key),
		Prove: true,
	})
	require.NoError(t, err)
	require.EqualValues(t, code.CodeTypeOK, resQuery.Code)
	require.Equal(t, key, string(resQuery.Key))
	require.Equal(t, value, string(resQuery.Value))
	require.EqualValues(t, info.LastBlockHeight, resQuery.Height)
}

func TestKVStoreKV(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kvstore := NewApplication()
	key := testKey
	value := key
	tx := []byte(key)
	testKVStore(ctx, t, kvstore, tx, key, value)

	value = testValue
	tx = []byte(key + "=" + value)
	testKVStore(ctx, t, kvstore, tx, key, value)
}

func TestPersistentKVStoreKV(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dir := t.TempDir()
	logger := log.NewNopLogger()

	kvstore := NewPersistentKVStoreApplication(logger, dir)
	key := testKey
	value := key
	tx := []byte(key)
	testKVStore(ctx, t, kvstore, tx, key, value)

	value = testValue
	tx = []byte(key + "=" + value)
	testKVStore(ctx, t, kvstore, tx, key, value)
}

func TestPersistentKVStoreInfo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dir := t.TempDir()
	logger := log.NewNopLogger()

	kvstore := NewPersistentKVStoreApplication(logger, dir)
	if err := InitKVStore(ctx, kvstore); err != nil {
		t.Fatal(err)
	}
	height := int64(0)

	resInfo, err := kvstore.Info(ctx, &types.RequestInfo{})
	if err != nil {
		t.Fatal(err)
	}

	if resInfo.LastBlockHeight != height {
		t.Fatalf("expected height of %d, got %d", height, resInfo.LastBlockHeight)
	}

	// make and apply block
	height = int64(1)
	hash := []byte("foo")
	if _, err := kvstore.FinalizeBlock(ctx, &types.RequestFinalizeBlock{Hash: hash, Height: height}); err != nil {
		t.Fatal(err)
	}

	if _, err := kvstore.Commit(ctx); err != nil {
		t.Fatal(err)

	}

	resInfo, err = kvstore.Info(ctx, &types.RequestInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if resInfo.LastBlockHeight != height {
		t.Fatalf("expected height of %d, got %d", height, resInfo.LastBlockHeight)
	}

}

// add a validator, remove a validator, update a validator
func TestValUpdates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kvstore := NewApplication()

	// init with some validators
	total := 10
	nInit := 5
	fullVals := RandValidatorSetUpdate(total)
	initVals := RandValidatorSetUpdate(nInit)

	// initialize with the first nInit
	_, err := kvstore.InitChain(ctx, &types.RequestInitChain{
		ValidatorSet: &initVals,
	})
	if err != nil {
		t.Fatal(err)
	}

	kvVals, err := kvstore.ValidatorSet()
	require.NoError(t, err)
	valSetEqualTest(t, kvVals, &initVals)

	tx, err := MarshalValidatorSetUpdate(&fullVals)
	require.NoError(t, err)

	// change the validator set to the full validator set
	makeApplyBlock(ctx, t, kvstore, 1, fullVals, tx)

	kvVals, err = kvstore.ValidatorSet()
	require.NoError(t, err)
	valSetEqualTest(t, kvVals, &fullVals)
}

func makeApplyBlock(
	ctx context.Context,
	t *testing.T,
	kvstore types.Application,
	heightInt int,
	diff types.ValidatorSetUpdate,
	txs ...[]byte) {
	// make and apply block
	height := int64(heightInt)
	hash := []byte("foo")
	resFinalizeBlock, err := kvstore.FinalizeBlock(ctx, &types.RequestFinalizeBlock{
		Hash:   hash,
		Height: height,
		Txs:    txs,
	})
	require.NoError(t, err)

	_, err = kvstore.Commit(ctx)
	require.NoError(t, err)

	valSetEqualTest(t, &diff, resFinalizeBlock.ValidatorSetUpdate)
}

// order doesn't matter
func valsEqualTest(t *testing.T, vals1, vals2 []types.ValidatorUpdate) {
	t.Helper()

	require.Equal(t, len(vals1), len(vals2), "vals dont match in len. got %d, expected %d", len(vals2), len(vals1))
	sort.Sort(types.ValidatorUpdates(vals1))
	sort.Sort(types.ValidatorUpdates(vals2))
	for i, v1 := range vals1 {
		v2 := vals2[i]
		if !v1.PubKey.Equal(v2.PubKey) ||
			v1.Power != v2.Power {
			t.Fatalf("vals dont match at index %d. got %X/%d , expected %X/%d", i, v2.PubKey, v2.Power, v1.PubKey, v1.Power)
		}
	}
}

func valSetEqualTest(t *testing.T, vals1, vals2 *types.ValidatorSetUpdate) {
	t.Helper()

	valsEqualTest(t, vals1.ValidatorUpdates, vals2.ValidatorUpdates)
	require.True(t,
		vals1.ThresholdPublicKey.Equal(vals2.ThresholdPublicKey),
		"val set threshold public key did not match. got %X, expected %X",
		vals1.ThresholdPublicKey, vals2.ThresholdPublicKey,
	)
	require.True(t,
		bytes.Equal(vals1.QuorumHash, vals2.QuorumHash),
		"val set quorum hash did not match. got %X, expected %X",
		vals1.QuorumHash, vals2.QuorumHash,
	)
}

func makeSocketClientServer(
	ctx context.Context,
	t *testing.T,
	logger log.Logger,
	app types.Application,
	name string,
) (abciclient.Client, service.Service, error) {
	t.Helper()

	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	t.Cleanup(leaktest.Check(t))

	// Start the listener
	socket := fmt.Sprintf("unix://%s.sock", name)

	server := abciserver.NewSocketServer(logger.With("module", "abci-server"), socket, app)
	if err := server.Start(ctx); err != nil {
		cancel()
		return nil, nil, err
	}

	// Connect to the socket
	client := abciclient.NewSocketClient(logger.With("module", "abci-client"), socket, false)
	if err := client.Start(ctx); err != nil {
		cancel()
		return nil, nil, err
	}

	return client, server, nil
}

func makeGRPCClientServer(
	ctx context.Context,
	t *testing.T,
	logger log.Logger,
	app types.Application,
	name string,
) (abciclient.Client, service.Service, error) {
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	t.Cleanup(leaktest.Check(t))

	// Start the listener
	socket := fmt.Sprintf("unix://%s.sock", name)

	server := abciserver.NewGRPCServer(logger.With("module", "abci-server"), socket, app)

	if err := server.Start(ctx); err != nil {
		cancel()
		return nil, nil, err
	}

	client := abciclient.NewGRPCClient(logger.With("module", "abci-client"), socket, true)

	if err := client.Start(ctx); err != nil {
		cancel()
		return nil, nil, err
	}
	return client, server, nil
}

func TestClientServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := log.NewNopLogger()

	// set up socket app
	kvstore := NewApplication()
	client, server, err := makeSocketClientServer(ctx, t, logger, kvstore, "kvstore-socket")
	require.NoError(t, err)
	t.Cleanup(func() { cancel(); server.Wait() })
	t.Cleanup(func() { cancel(); client.Wait() })

	runClientTests(ctx, t, client)

	// set up grpc app
	kvstore = NewApplication()
	gclient, gserver, err := makeGRPCClientServer(ctx, t, logger, kvstore, "/tmp/kvstore-grpc")
	require.NoError(t, err)

	t.Cleanup(func() { cancel(); gserver.Wait() })
	t.Cleanup(func() { cancel(); gclient.Wait() })

	runClientTests(ctx, t, gclient)
}

func runClientTests(ctx context.Context, t *testing.T, client abciclient.Client) {
	// run some tests....
	key := testKey
	value := key
	tx := []byte(key)
	testClient(ctx, t, client, tx, key, value)

	value = testValue
	tx = []byte(key + "=" + value)
	testClient(ctx, t, client, tx, key, value)
}

func testClient(ctx context.Context, t *testing.T, app abciclient.Client, tx []byte, key, value string) {
	ar, err := app.FinalizeBlock(ctx, &types.RequestFinalizeBlock{Txs: [][]byte{tx}})
	require.NoError(t, err)
	require.Equal(t, 1, len(ar.TxResults))
	require.False(t, ar.TxResults[0].IsErr())
	// repeating FinalizeBlock doesn't raise error
	ar, err = app.FinalizeBlock(ctx, &types.RequestFinalizeBlock{Txs: [][]byte{tx}})
	require.NoError(t, err)
	require.Equal(t, 1, len(ar.TxResults))
	require.False(t, ar.TxResults[0].IsErr())
	// commit
	_, err = app.Commit(ctx)
	require.NoError(t, err)

	info, err := app.Info(ctx, &types.RequestInfo{})
	require.NoError(t, err)
	require.NotZero(t, info.LastBlockHeight)

	// make sure query is fine
	resQuery, err := app.Query(ctx, &types.RequestQuery{
		Path: "/store",
		Data: []byte(key),
	})
	require.NoError(t, err)
	require.Equal(t, code.CodeTypeOK, resQuery.Code)
	require.Equal(t, key, string(resQuery.Key))
	require.Equal(t, value, string(resQuery.Value))
	require.EqualValues(t, info.LastBlockHeight, resQuery.Height)

	// make sure proof is fine
	resQuery, err = app.Query(ctx, &types.RequestQuery{
		Path:  "/store",
		Data:  []byte(key),
		Prove: true,
	})
	require.NoError(t, err)
	require.Equal(t, code.CodeTypeOK, resQuery.Code)
	require.Equal(t, key, string(resQuery.Key))
	require.Equal(t, value, string(resQuery.Value))
	require.EqualValues(t, info.LastBlockHeight, resQuery.Height)
}
