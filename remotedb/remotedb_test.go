package remotedb_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tmlibs/grpcdb"
	"github.com/tendermint/tmlibs/remotedb"
)

func TestRemoteDB(t *testing.T) {
	cert := "::.crt"
	key := "::.key"
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	require.Nil(t, err, "expecting a port to have been assigned on which we can listen")
	srv, err := grpcdb.NewServer(cert, key)
	require.Nil(t, err)
	defer srv.Stop()
	go func() {
		if err := srv.Serve(ln); err != nil {
			t.Fatalf("BindServer: %v", err)
		}
	}()

	client, err := remotedb.NewRemoteDB(ln.Addr().String(), cert)
	require.Nil(t, err, "expecting a successful client creation")
	require.Nil(t, client.InitRemote(&remotedb.Init{Name: "test-remote-db", Type: "leveldb"}))

	k1 := []byte("key-1")
	v1 := client.Get(k1)
	require.Equal(t, 0, len(v1), "expecting no key1 to have been stored")
	vv1 := []byte("value-1")
	client.Set(k1, vv1)
	gv1 := client.Get(k1)
	require.Equal(t, gv1, vv1)

	// Simple iteration
	itr := client.Iterator(nil, nil)
	itr.Next()
	require.Equal(t, itr.Key(), []byte("key-1"))
	require.Equal(t, itr.Value(), []byte("value-1"))
	require.Panics(t, itr.Next)
	itr.Close()

	// Set some more keys
	k2 := []byte("key-2")
	v2 := []byte("value-2")
	client.Set(k2, v2)
	gv2 := client.Get(k2)
	require.Equal(t, gv2, v2)

	// More iteration
	itr = client.Iterator(nil, nil)
	itr.Next()
	require.Equal(t, itr.Key(), []byte("key-1"))
	require.Equal(t, itr.Value(), []byte("value-1"))
	itr.Next()
	require.Equal(t, itr.Key(), []byte("key-2"))
	require.Equal(t, itr.Value(), []byte("value-2"))
	require.Panics(t, itr.Next)

	// Deletion
	client.Delete(k1)
	client.Delete(k2)
	gv1 = client.Get(k1)
	gv2 = client.Get(k2)
	require.Equal(t, len(gv2), 0, "after deletion, not expecting the key to exist anymore")
	require.Equal(t, len(gv1), 0, "after deletion, not expecting the key to exist anymore")

	// TODO Batch tests
}
