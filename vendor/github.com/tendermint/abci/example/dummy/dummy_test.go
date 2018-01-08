package dummy

import (
	"bytes"
	"io/ioutil"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/iavl"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"

	abcicli "github.com/tendermint/abci/client"
	"github.com/tendermint/abci/example/code"
	abciserver "github.com/tendermint/abci/server"
	"github.com/tendermint/abci/types"
)

func testDummy(t *testing.T, app types.Application, tx []byte, key, value string) {
	ar := app.DeliverTx(tx)
	require.False(t, ar.IsErr(), ar)
	// repeating tx doesn't raise error
	ar = app.DeliverTx(tx)
	require.False(t, ar.IsErr(), ar)

	// make sure query is fine
	resQuery := app.Query(types.RequestQuery{
		Path: "/store",
		Data: []byte(key),
	})
	require.Equal(t, code.CodeTypeOK, resQuery.Code)
	require.Equal(t, value, string(resQuery.Value))

	// make sure proof is fine
	resQuery = app.Query(types.RequestQuery{
		Path:  "/store",
		Data:  []byte(key),
		Prove: true,
	})
	require.EqualValues(t, code.CodeTypeOK, resQuery.Code)
	require.Equal(t, value, string(resQuery.Value))
	proof, err := iavl.ReadKeyExistsProof(resQuery.Proof)
	require.Nil(t, err)
	err = proof.Verify([]byte(key), resQuery.Value, proof.RootHash)
	require.Nil(t, err, "%+v", err) // NOTE: we have no way to verify the RootHash
}

func TestDummyKV(t *testing.T) {
	dummy := NewDummyApplication()
	key := "abc"
	value := key
	tx := []byte(key)
	testDummy(t, dummy, tx, key, value)

	value = "def"
	tx = []byte(key + "=" + value)
	testDummy(t, dummy, tx, key, value)
}

func TestPersistentDummyKV(t *testing.T) {
	dir, err := ioutil.TempDir("/tmp", "abci-dummy-test") // TODO
	if err != nil {
		t.Fatal(err)
	}
	dummy := NewPersistentDummyApplication(dir)
	key := "abc"
	value := key
	tx := []byte(key)
	testDummy(t, dummy, tx, key, value)

	value = "def"
	tx = []byte(key + "=" + value)
	testDummy(t, dummy, tx, key, value)
}

func TestPersistentDummyInfo(t *testing.T) {
	dir, err := ioutil.TempDir("/tmp", "abci-dummy-test") // TODO
	if err != nil {
		t.Fatal(err)
	}
	dummy := NewPersistentDummyApplication(dir)
	InitDummy(dummy)
	height := int64(0)

	resInfo := dummy.Info(types.RequestInfo{})
	if resInfo.LastBlockHeight != height {
		t.Fatalf("expected height of %d, got %d", height, resInfo.LastBlockHeight)
	}

	// make and apply block
	height = int64(1)
	hash := []byte("foo")
	header := &types.Header{
		Height: int64(height),
	}
	dummy.BeginBlock(types.RequestBeginBlock{hash, header, nil, nil})
	dummy.EndBlock(types.RequestEndBlock{header.Height})
	dummy.Commit()

	resInfo = dummy.Info(types.RequestInfo{})
	if resInfo.LastBlockHeight != height {
		t.Fatalf("expected height of %d, got %d", height, resInfo.LastBlockHeight)
	}

}

// add a validator, remove a validator, update a validator
func TestValUpdates(t *testing.T) {
	dir, err := ioutil.TempDir("/tmp", "abci-dummy-test") // TODO
	if err != nil {
		t.Fatal(err)
	}
	dummy := NewPersistentDummyApplication(dir)

	// init with some validators
	total := 10
	nInit := 5
	vals := RandVals(total)
	// iniitalize with the first nInit
	dummy.InitChain(types.RequestInitChain{vals[:nInit]})

	vals1, vals2 := vals[:nInit], dummy.Validators()
	valsEqual(t, vals1, vals2)

	var v1, v2, v3 *types.Validator

	// add some validators
	v1, v2 = vals[nInit], vals[nInit+1]
	diff := []*types.Validator{v1, v2}
	tx1 := MakeValSetChangeTx(v1.PubKey, v1.Power)
	tx2 := MakeValSetChangeTx(v2.PubKey, v2.Power)

	makeApplyBlock(t, dummy, 1, diff, tx1, tx2)

	vals1, vals2 = vals[:nInit+2], dummy.Validators()
	valsEqual(t, vals1, vals2)

	// remove some validators
	v1, v2, v3 = vals[nInit-2], vals[nInit-1], vals[nInit]
	v1.Power = 0
	v2.Power = 0
	v3.Power = 0
	diff = []*types.Validator{v1, v2, v3}
	tx1 = MakeValSetChangeTx(v1.PubKey, v1.Power)
	tx2 = MakeValSetChangeTx(v2.PubKey, v2.Power)
	tx3 := MakeValSetChangeTx(v3.PubKey, v3.Power)

	makeApplyBlock(t, dummy, 2, diff, tx1, tx2, tx3)

	vals1 = append(vals[:nInit-2], vals[nInit+1])
	vals2 = dummy.Validators()
	valsEqual(t, vals1, vals2)

	// update some validators
	v1 = vals[0]
	if v1.Power == 5 {
		v1.Power = 6
	} else {
		v1.Power = 5
	}
	diff = []*types.Validator{v1}
	tx1 = MakeValSetChangeTx(v1.PubKey, v1.Power)

	makeApplyBlock(t, dummy, 3, diff, tx1)

	vals1 = append([]*types.Validator{v1}, vals1[1:]...)
	vals2 = dummy.Validators()
	valsEqual(t, vals1, vals2)

}

func makeApplyBlock(t *testing.T, dummy types.Application, heightInt int, diff []*types.Validator, txs ...[]byte) {
	// make and apply block
	height := int64(heightInt)
	hash := []byte("foo")
	header := &types.Header{
		Height: height,
	}

	dummy.BeginBlock(types.RequestBeginBlock{hash, header, nil, nil})
	for _, tx := range txs {
		if r := dummy.DeliverTx(tx); r.IsErr() {
			t.Fatal(r)
		}
	}
	resEndBlock := dummy.EndBlock(types.RequestEndBlock{header.Height})
	dummy.Commit()

	valsEqual(t, diff, resEndBlock.ValidatorUpdates)

}

// order doesn't matter
func valsEqual(t *testing.T, vals1, vals2 []*types.Validator) {
	if len(vals1) != len(vals2) {
		t.Fatalf("vals dont match in len. got %d, expected %d", len(vals2), len(vals1))
	}
	sort.Sort(types.Validators(vals1))
	sort.Sort(types.Validators(vals2))
	for i, v1 := range vals1 {
		v2 := vals2[i]
		if !bytes.Equal(v1.PubKey, v2.PubKey) ||
			v1.Power != v2.Power {
			t.Fatalf("vals dont match at index %d. got %X/%d , expected %X/%d", i, v2.PubKey, v2.Power, v1.PubKey, v1.Power)
		}
	}
}

func makeSocketClientServer(app types.Application, name string) (abcicli.Client, cmn.Service, error) {
	// Start the listener
	socket := cmn.Fmt("unix://%s.sock", name)
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
		server.Stop()
		return nil, nil, err
	}

	return client, server, nil
}

func makeGRPCClientServer(app types.Application, name string) (abcicli.Client, cmn.Service, error) {
	// Start the listener
	socket := cmn.Fmt("unix://%s.sock", name)
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
		server.Stop()
		return nil, nil, err
	}
	return client, server, nil
}

func TestClientServer(t *testing.T) {
	// set up socket app
	dummy := NewDummyApplication()
	client, server, err := makeSocketClientServer(dummy, "dummy-socket")
	require.Nil(t, err)
	defer server.Stop()
	defer client.Stop()

	runClientTests(t, client)

	// set up grpc app
	dummy = NewDummyApplication()
	gclient, gserver, err := makeGRPCClientServer(dummy, "dummy-grpc")
	require.Nil(t, err)
	defer gserver.Stop()
	defer gclient.Stop()

	runClientTests(t, gclient)
}

func runClientTests(t *testing.T, client abcicli.Client) {
	// run some tests....
	key := "abc"
	value := key
	tx := []byte(key)
	testClient(t, client, tx, key, value)

	value = "def"
	tx = []byte(key + "=" + value)
	testClient(t, client, tx, key, value)
}

func testClient(t *testing.T, app abcicli.Client, tx []byte, key, value string) {
	ar, err := app.DeliverTxSync(tx)
	require.NoError(t, err)
	require.False(t, ar.IsErr(), ar)
	// repeating tx doesn't raise error
	ar, err = app.DeliverTxSync(tx)
	require.NoError(t, err)
	require.False(t, ar.IsErr(), ar)

	// make sure query is fine
	resQuery, err := app.QuerySync(types.RequestQuery{
		Path: "/store",
		Data: []byte(key),
	})
	require.Nil(t, err)
	require.Equal(t, code.CodeTypeOK, resQuery.Code)
	require.Equal(t, value, string(resQuery.Value))

	// make sure proof is fine
	resQuery, err = app.QuerySync(types.RequestQuery{
		Path:  "/store",
		Data:  []byte(key),
		Prove: true,
	})
	require.Nil(t, err)
	require.Equal(t, code.CodeTypeOK, resQuery.Code)
	require.Equal(t, value, string(resQuery.Value))
	proof, err := iavl.ReadKeyExistsProof(resQuery.Proof)
	require.Nil(t, err)
	err = proof.Verify([]byte(key), resQuery.Value, proof.RootHash)
	require.Nil(t, err, "%+v", err) // NOTE: we have no way to verify the RootHash
}
