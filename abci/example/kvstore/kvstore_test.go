package kvstore

import (
	"fmt"
	"io/ioutil"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"

	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/code"
	abciserver "github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/abci/types"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

const (
	testKey   = "abc"
	testValue = "def"
)

func testKVStore(t *testing.T, app types.Application, tx []byte, key, value string) {
	req := types.RequestDeliverTx{Tx: tx}
	ar := app.DeliverTx(req)
	require.False(t, ar.IsErr(), ar)
	// repeating tx doesn't raise error
	ar = app.DeliverTx(req)
	require.False(t, ar.IsErr(), ar)
	// commit
	app.Commit()

	info := app.Info(types.RequestInfo{})
	require.NotZero(t, info.LastBlockHeight)

	// make sure query is fine
	resQuery := app.Query(types.RequestQuery{
		Path: "/store",
		Data: []byte(key),
	})
	require.Equal(t, code.CodeTypeOK, resQuery.Code)
	require.Equal(t, key, string(resQuery.Key))
	require.Equal(t, value, string(resQuery.Value))
	require.EqualValues(t, info.LastBlockHeight, resQuery.Height)

	// make sure proof is fine
	resQuery = app.Query(types.RequestQuery{
		Path:  "/store",
		Data:  []byte(key),
		Prove: true,
	})
	require.EqualValues(t, code.CodeTypeOK, resQuery.Code)
	require.Equal(t, key, string(resQuery.Key))
	require.Equal(t, value, string(resQuery.Value))
	require.EqualValues(t, info.LastBlockHeight, resQuery.Height)
}

func TestKVStoreKV(t *testing.T) {
	kvstore := NewApplication()
	key := testKey
	value := key
	tx := []byte(key)
	testKVStore(t, kvstore, tx, key, value)

	value = testValue
	tx = []byte(key + "=" + value)
	testKVStore(t, kvstore, tx, key, value)
}

func TestPersistentKVStoreKV(t *testing.T) {
	dir, err := ioutil.TempDir("/tmp", "abci-kvstore-test") // TODO
	if err != nil {
		t.Fatal(err)
	}
	kvstore := NewPersistentKVStoreApplication(dir)
	key := testKey
	value := key
	tx := []byte(key)
	testKVStore(t, kvstore, tx, key, value)

	value = testValue
	tx = []byte(key + "=" + value)
	testKVStore(t, kvstore, tx, key, value)
}

func TestPersistentKVStoreInfo(t *testing.T) {
	dir, err := ioutil.TempDir("/tmp", "abci-kvstore-test") // TODO
	if err != nil {
		t.Fatal(err)
	}
	kvstore := NewPersistentKVStoreApplication(dir)
	InitKVStore(kvstore)
	height := int64(0)

	resInfo := kvstore.Info(types.RequestInfo{})
	if resInfo.LastBlockHeight != height {
		t.Fatalf("expected height of %d, got %d", height, resInfo.LastBlockHeight)
	}

	// make and apply block
	height = int64(1)
	hash := []byte("foo")
	header := tmproto.Header{
		Height: height,
	}
	kvstore.BeginBlock(types.RequestBeginBlock{Hash: hash, Header: header})
	kvstore.EndBlock(types.RequestEndBlock{Height: header.Height})
	kvstore.Commit()

	resInfo = kvstore.Info(types.RequestInfo{})
	if resInfo.LastBlockHeight != height {
		t.Fatalf("expected height of %d, got %d", height, resInfo.LastBlockHeight)
	}

}

// add a validator, remove a validator, update a validator
func TestValUpdates(t *testing.T) {
	dir, err := ioutil.TempDir("/tmp", "abci-kvstore-test") // TODO
	if err != nil {
		t.Fatal(err)
	}
	kvstore := NewPersistentKVStoreApplication(dir)

	// init with some validators
	total := 10
	nInit := 5
	vals := RandVals(total)
	// initialize with the first nInit
	kvstore.InitChain(types.RequestInitChain{
		Validators: vals[:nInit],
	})

	vals1, vals2 := vals[:nInit], kvstore.Validators()
	valsEqual(t, vals1, vals2)

	var v1, v2, v3 types.ValidatorUpdate

	// add some validators
	v1, v2 = vals[nInit], vals[nInit+1]
	diff := []types.ValidatorUpdate{v1, v2}
	tx1 := MakeValSetChangeTx(v1.PubKey, v1.Power)
	tx2 := MakeValSetChangeTx(v2.PubKey, v2.Power)

	makeApplyBlock(t, kvstore, 1, diff, tx1, tx2)

	vals1, vals2 = vals[:nInit+2], kvstore.Validators()
	valsEqual(t, vals1, vals2)

	// remove some validators
	v1, v2, v3 = vals[nInit-2], vals[nInit-1], vals[nInit]
	v1.Power = 0
	v2.Power = 0
	v3.Power = 0
	diff = []types.ValidatorUpdate{v1, v2, v3}
	tx1 = MakeValSetChangeTx(v1.PubKey, v1.Power)
	tx2 = MakeValSetChangeTx(v2.PubKey, v2.Power)
	tx3 := MakeValSetChangeTx(v3.PubKey, v3.Power)

	makeApplyBlock(t, kvstore, 2, diff, tx1, tx2, tx3)

	vals1 = append(vals[:nInit-2], vals[nInit+1]) // nolint: gocritic
	vals2 = kvstore.Validators()
	valsEqual(t, vals1, vals2)

	// update some validators
	v1 = vals[0]
	if v1.Power == 5 {
		v1.Power = 6
	} else {
		v1.Power = 5
	}
	diff = []types.ValidatorUpdate{v1}
	tx1 = MakeValSetChangeTx(v1.PubKey, v1.Power)

	makeApplyBlock(t, kvstore, 3, diff, tx1)

	vals1 = append([]types.ValidatorUpdate{v1}, vals1[1:]...)
	vals2 = kvstore.Validators()
	valsEqual(t, vals1, vals2)

}

func makeApplyBlock(
	t *testing.T,
	kvstore types.Application,
	heightInt int,
	diff []types.ValidatorUpdate,
	txs ...[]byte) {
	// make and apply block
	height := int64(heightInt)
	hash := []byte("foo")
	header := tmproto.Header{
		Height: height,
	}

	kvstore.BeginBlock(types.RequestBeginBlock{Hash: hash, Header: header})
	for _, tx := range txs {
		if r := kvstore.DeliverTx(types.RequestDeliverTx{Tx: tx}); r.IsErr() {
			t.Fatal(r)
		}
	}
	resEndBlock := kvstore.EndBlock(types.RequestEndBlock{Height: header.Height})
	kvstore.Commit()

	valsEqual(t, diff, resEndBlock.ValidatorUpdates)

}

// order doesn't matter
func valsEqual(t *testing.T, vals1, vals2 []types.ValidatorUpdate) {
	if len(vals1) != len(vals2) {
		t.Fatalf("vals dont match in len. got %d, expected %d", len(vals2), len(vals1))
	}
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

func makeSocketClientServer(app types.Application, name string) (abcicli.Client, service.Service, error) {
	// Start the listener
	socket := fmt.Sprintf("unix://%s.sock", name)
	logger := log.TestingLogger()

	server := abciserver.NewSocketServer(socket, app)
	server.SetLogger(logger.With("module", "abci-server"))
	if err := server.Start(); err != nil {
		return nil, nil, err
	}

	// Connect to the socket
	client := abcicli.NewSocketClient(socket, false)
	client.SetLogger(logger.With("module", "abci-client"))
	if err := client.Start(); err != nil {
		if err = server.Stop(); err != nil {
			return nil, nil, err
		}
		return nil, nil, err
	}

	return client, server, nil
}

func makeGRPCClientServer(app types.Application, name string) (abcicli.Client, service.Service, error) {
	// Start the listener
	socket := fmt.Sprintf("unix://%s.sock", name)
	logger := log.TestingLogger()

	gapp := types.NewGRPCApplication(app)
	server := abciserver.NewGRPCServer(socket, gapp)
	server.SetLogger(logger.With("module", "abci-server"))
	if err := server.Start(); err != nil {
		return nil, nil, err
	}

	client := abcicli.NewGRPCClient(socket, true)
	client.SetLogger(logger.With("module", "abci-client"))
	if err := client.Start(); err != nil {
		if err := server.Stop(); err != nil {
			return nil, nil, err
		}
		return nil, nil, err
	}
	return client, server, nil
}

func TestClientServer(t *testing.T) {
	// set up socket app
	kvstore := NewApplication()
	client, server, err := makeSocketClientServer(kvstore, "kvstore-socket")
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := server.Stop(); err != nil {
			t.Error(err)
		}
	})
	t.Cleanup(func() {
		if err := client.Stop(); err != nil {
			t.Error(err)
		}
	})

	runClientTests(t, client)

	// set up grpc app
	kvstore = NewApplication()
	gclient, gserver, err := makeGRPCClientServer(kvstore, "kvstore-grpc")
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := gserver.Stop(); err != nil {
			t.Error(err)
		}
	})
	t.Cleanup(func() {
		if err := gclient.Stop(); err != nil {
			t.Error(err)
		}
	})

	runClientTests(t, gclient)
}

func runClientTests(t *testing.T, client abcicli.Client) {
	// run some tests....
	key := testKey
	value := key
	tx := []byte(key)
	testClient(t, client, tx, key, value)

	value = testValue
	tx = []byte(key + "=" + value)
	testClient(t, client, tx, key, value)
}

func testClient(t *testing.T, app abcicli.Client, tx []byte, key, value string) {
	ar, err := app.DeliverTxSync(types.RequestDeliverTx{Tx: tx})
	require.NoError(t, err)
	require.False(t, ar.IsErr(), ar)
	// repeating tx doesn't raise error
	ar, err = app.DeliverTxSync(types.RequestDeliverTx{Tx: tx})
	require.NoError(t, err)
	require.False(t, ar.IsErr(), ar)
	// commit
	_, err = app.CommitSync()
	require.NoError(t, err)

	info, err := app.InfoSync(types.RequestInfo{})
	require.NoError(t, err)
	require.NotZero(t, info.LastBlockHeight)

	// make sure query is fine
	resQuery, err := app.QuerySync(types.RequestQuery{
		Path: "/store",
		Data: []byte(key),
	})
	require.Nil(t, err)
	require.Equal(t, code.CodeTypeOK, resQuery.Code)
	require.Equal(t, key, string(resQuery.Key))
	require.Equal(t, value, string(resQuery.Value))
	require.EqualValues(t, info.LastBlockHeight, resQuery.Height)

	// make sure proof is fine
	resQuery, err = app.QuerySync(types.RequestQuery{
		Path:  "/store",
		Data:  []byte(key),
		Prove: true,
	})
	require.Nil(t, err)
	require.Equal(t, code.CodeTypeOK, resQuery.Code)
	require.Equal(t, key, string(resQuery.Key))
	require.Equal(t, value, string(resQuery.Value))
	require.EqualValues(t, info.LastBlockHeight, resQuery.Height)
}
