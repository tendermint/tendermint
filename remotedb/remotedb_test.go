package remotedb_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tmlibs/grpcdb"
	"github.com/tendermint/tmlibs/remotedb"
)

func TestRemoteDB(t *testing.T) {
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	require.Nil(t, err, "expecting a port to have been assigned on which we can listen")
	srv := grpcdb.NewServer()
	defer srv.Stop()
	go func() {
		if err := srv.Serve(ln); err != nil {
			t.Fatalf("BindServer: %v", err)
		}
	}()

	client, err := remotedb.NewInsecure(ln.Addr().String())
	require.Nil(t, err, "expecting a successful client creation")
	require.Nil(t, client.InitRemote(&remotedb.Init{Name: "test-remote-db", Type: "leveldb"}))

	k1 := []byte("key-1")
	v1 := client.Get(k1)
	require.Equal(t, 0, len(v1), "expecting no key1 to have been stored")
	vv1 := []byte("value-1")
	client.Set(k1, vv1)
	gv1 := client.Get(k1)
	require.Equal(t, gv1, vv1)

	// Deletion
	client.Delete(k1)
	gv2 := client.Get(k1)
	require.Equal(t, len(gv2), 0, "after deletion, not expecting the key to exist anymore")
	require.NotEqual(t, len(gv1), len(gv2), "after deletion, not expecting the key to exist anymore")
}
