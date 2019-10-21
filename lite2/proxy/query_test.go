package proxy

import (
	"fmt"
	"os"
	"testing"
	"time"

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
//nolint:unused
var waitForEventTimeout = 5 * time.Second

// TODO fix tests!!

func TestMain(m *testing.M) {
	app := kvstore.NewKVStoreApplication()
	node = rpctest.StartTendermint(app)

	code := m.Run()

	rpctest.StopTendermint(node)
	os.Exit(code)
}

func kvstoreTx(k, v []byte) []byte {
	return []byte(fmt.Sprintf("%s=%s", k, v))
}

// TODO: enable it after general proof format has been adapted
// in abci/examples/kvstore.go
//nolint:unused,deadcode
func _TestAppProofs(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	prt := defaultProofRuntime()
	cl := client.NewLocal(node)
	client.WaitForHeight(cl, 1, nil)

	// This sets up our trust on the node based on some past point.
	source := certclient.NewProvider(chainID, cl)
	seed, err := source.LatestFullCommit(chainID, 1, 1)
	require.NoError(err, "%#v", err)
	cert := lite.NewBaseVerifier(chainID, seed.Height(), seed.Validators)

	// Wait for tx confirmation.
	done := make(chan int64)
	go func() {
		evtTyp := types.EventTx
		_, err = client.WaitForOneEvent(cl, evtTyp, waitForEventTimeout)
		require.Nil(err, "%#v", err)
		close(done)
	}()

	// Submit a transaction.
	k := []byte("my-key")
	v := []byte("my-value")
	tx := kvstoreTx(k, v)
	br, err := cl.BroadcastTxCommit(tx)
	require.NoError(err, "%#v", err)
	require.EqualValues(0, br.CheckTx.Code, "%#v", br.CheckTx)
	require.EqualValues(0, br.DeliverTx.Code)
	brh := br.Height

	// Fetch latest after tx commit.
	<-done
	latest, err := source.LatestFullCommit(chainID, 1, 1<<63-1)
	require.NoError(err, "%#v", err)
	rootHash := latest.SignedHeader.AppHash
	if rootHash == nil {
		// Fetch one block later, AppHash hasn't been committed yet.
		// TODO find a way to avoid doing this.
		client.WaitForHeight(cl, latest.SignedHeader.Height+1, nil)
		latest, err = source.LatestFullCommit(chainID, latest.SignedHeader.Height+1, 1<<63-1)
		require.NoError(err, "%#v", err)
		rootHash = latest.SignedHeader.AppHash
	}
	require.NotNil(rootHash)

	// verify a query before the tx block has no data (and valid non-exist proof)
	bs, height, proof, err := GetWithProof(prt, k, brh-1, cl, cert)
	require.NoError(err, "%#v", err)
	require.NotNil(proof)
	require.Equal(height, brh-1)
	// require.NotNil(proof)
	// TODO: Ensure that *some* keys will be there, ensuring that proof is nil,
	// (currently there's a race condition)
	// and ensure that proof proves absence of k.
	require.Nil(bs)

	// but given that block it is good
	bs, height, proof, err = GetWithProof(prt, k, brh, cl, cert)
	require.NoError(err, "%#v", err)
	require.NotNil(proof)
	require.Equal(height, brh)

	assert.EqualValues(v, bs)
	err = prt.VerifyValue(proof, rootHash, string(k), bs) // XXX key encoding
	assert.NoError(err, "%#v", err)

	// Test non-existing key.
	missing := []byte("my-missing-key")
	bs, _, proof, err = GetWithProof(prt, missing, 0, cl, cert)
	require.NoError(err)
	require.Nil(bs)
	require.NotNil(proof)
	err = prt.VerifyAbsence(proof, rootHash, string(missing)) // XXX VerifyAbsence(), keyencoding
	assert.NoError(err, "%#v", err)
	err = prt.VerifyAbsence(proof, rootHash, string(k)) // XXX VerifyAbsence(), keyencoding
	assert.Error(err, "%#v", err)
}

func TestTxProofs(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	cl := client.NewLocal(node)
	client.WaitForHeight(cl, 1, nil)

	tx := kvstoreTx([]byte("key-a"), []byte("value-a"))
	br, err := cl.BroadcastTxCommit(tx)
	require.NoError(err, "%#v", err)
	require.EqualValues(0, br.CheckTx.Code, "%#v", br.CheckTx)
	require.EqualValues(0, br.DeliverTx.Code)
	brh := br.Height

	source := certclient.NewProvider(chainID, cl)
	seed, err := source.LatestFullCommit(chainID, brh-2, brh-2)
	require.NoError(err, "%#v", err)
	cert := lite.NewBaseVerifier(chainID, seed.Height(), seed.Validators)

	// First let's make sure a bogus transaction hash returns a valid non-existence proof.
	key := types.Tx([]byte("bogus")).Hash()
	_, err = cl.Tx(key, true)
	require.NotNil(err)
	require.Contains(err.Error(), "not found")

	// Now let's check with the real tx root hash.
	key = types.Tx(tx).Hash()
	res, err := cl.Tx(key, true)
	require.NoError(err, "%#v", err)
	require.NotNil(res)
	keyHash := merkle.SimpleHashFromByteSlices([][]byte{key})
	err = res.Proof.Validate(keyHash)
	assert.NoError(err, "%#v", err)

	commit, err := GetCertifiedCommit(br.Height, cl, cert)
	require.Nil(err, "%#v", err)
	require.Equal(res.Proof.RootHash, commit.Header.DataHash)
}
