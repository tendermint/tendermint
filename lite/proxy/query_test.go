package proxy

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/abci/example/kvstore"

	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/lite"
	certclient "github.com/tendermint/tendermint/lite/client"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/rpc/client"
	rpctest "github.com/tendermint/tendermint/rpc/test"
	"github.com/tendermint/tendermint/types"
)

var node *nm.Node
var chainID = "tendermint_test" // TODO use from config.

// TODO fix tests!!

func TestMain(m *testing.M) {
	app := kvstore.NewKVStoreApplication()
	node = rpctest.StartTendermint(app)

	code := m.Run()

	node.Stop()
	node.Wait()
	os.Exit(code)
}

func kvstoreTx(k, v []byte) []byte {
	return []byte(fmt.Sprintf("%s=%s", k, v))
}

func TestAppProofs(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	cl := client.NewLocal(node)
	client.WaitForHeight(cl, 1, nil)

	k := []byte("my-key")
	v := []byte("my-value")

	tx := kvstoreTx(k, v)
	br, err := cl.BroadcastTxCommit(tx)
	require.NoError(err, "%+v", err)
	require.EqualValues(0, br.CheckTx.Code, "%#v", br.CheckTx)
	require.EqualValues(0, br.DeliverTx.Code)
	brh := br.Height

	// This sets up our trust on the node based on some past point.
	source := certclient.NewProvider(chainID, cl)
	seed, err := source.LatestFullCommit(chainID, brh-2, brh-2)
	require.NoError(err, "%+v", err)
	cert := lite.NewBaseCertifier("my-chain", seed.Height(), seed.Validators)

	client.WaitForHeight(cl, 3, nil)
	latest, err := source.LatestFullCommit(chainID, 1, 1<<63-1)
	require.NoError(err, "%+v", err)
	rootHash := latest.SignedHeader.AppHash
	require.NotNil(rootHash)

	// verify a query before the tx block has no data (and valid non-exist proof)
	bs, height, proof, err := GetWithProof(k, brh-1, cl, cert)
	require.NotNil(err)
	require.NotNil(proof)
	// XXX test proof
	require.True(IsErrNoData(err), err.Error())
	require.Nil(bs)

	// but given that block it is good
	bs, height, proof, err = GetWithProof(k, brh, cl, cert)
	require.NoError(err, "%+v", err)
	require.NotNil(proof)
	require.True(height >= int64(latest.Height()))

	prt := merkle.NewProofRuntime()

	assert.EqualValues(v, bs)
	err = prt.VerifyValue(proof, rootHash, k, string(bs))
	assert.NoError(err, "%+v", err)

	// Test non-existing key.
	missing := []byte("my-missing-key")
	bs, _, proof, err = GetWithProof(missing, 0, cl, cert)
	require.True(IsErrNoData(err))
	require.Nil(bs)
	require.NotNil(proof)
	err = prt.VerifyValue(proof, rootHash, missing, "") // XXX VerifyAbsence()
	assert.NoError(err, "%+v", err)
	err = prt.VerifyValue(proof, rootHash, k, "") // XXX VerifyAbsence()
	assert.Error(err)
}

func _TestTxProofs(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	cl := client.NewLocal(node)
	client.WaitForHeight(cl, 1, nil)

	tx := kvstoreTx([]byte("key-a"), []byte("value-a"))
	br, err := cl.BroadcastTxCommit(tx)
	require.NoError(err, "%+v", err)
	require.EqualValues(0, br.CheckTx.Code, "%#v", br.CheckTx)
	require.EqualValues(0, br.DeliverTx.Code)
	brh := br.Height

	source := certclient.NewProvider(chainID, cl)
	seed, err := source.LatestFullCommit(chainID, brh-2, brh-2)
	require.NoError(err, "%+v", err)
	cert := lite.NewBaseCertifier("my-chain", seed.Height(), seed.Validators)

	// First let's make sure a bogus transaction hash returns a valid non-existence proof.
	key := types.Tx([]byte("bogus")).Hash()
	res, err := cl.Tx(key, true)
	require.NotNil(err)
	require.Contains(err.Error(), "not found")

	// Now let's check with the real tx hash.
	key = types.Tx(tx).Hash()
	res, err = cl.Tx(key, true)
	require.NoError(err, "%+v", err)
	require.NotNil(res)
	err = res.Proof.Validate(key)
	assert.NoError(err, "%+v", err)

	commit, err := GetCertifiedCommit(br.Height, cl, cert)
	require.Nil(err, "%+v", err)
	require.Equal(res.Proof.RootHash, commit.Header.DataHash)
}
