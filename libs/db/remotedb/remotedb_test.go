package remotedb_test

import (
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/db/remotedb"
	"github.com/tendermint/tendermint/libs/db/remotedb/grpcdb"
)

func TestRemoteDB(t *testing.T) {
	cert := "test.crt"
	key := "test.key"
	ln, err := net.Listen("tcp", "localhost:0")
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
	dbName := "test-remote-db"
	require.Nil(t, client.InitRemote(&remotedb.Init{Name: dbName, Type: "leveldb"}))
	defer func() {
		err := os.RemoveAll(dbName + ".db")
		if err != nil {
			panic(err)
		}
	}()

	k1 := []byte("key-1")
	v1 := client.Get(k1)
	require.Equal(t, 0, len(v1), "expecting no key1 to have been stored, got %X (%s)", v1, v1)
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
	client.SetSync(k2, v2)
	has := client.Has(k2)
	require.True(t, has)
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
	itr.Close()

	// Deletion
	client.Delete(k1)
	client.DeleteSync(k2)
	gv1 = client.Get(k1)
	gv2 = client.Get(k2)
	require.Equal(t, len(gv2), 0, "after deletion, not expecting the key to exist anymore")
	require.Equal(t, len(gv1), 0, "after deletion, not expecting the key to exist anymore")

	// Batch tests - set
	k3 := []byte("key-3")
	k4 := []byte("key-4")
	k5 := []byte("key-5")
	v3 := []byte("value-3")
	v4 := []byte("value-4")
	v5 := []byte("value-5")
	bat := client.NewBatch()
	bat.Set(k3, v3)
	bat.Set(k4, v4)
	rv3 := client.Get(k3)
	require.Equal(t, 0, len(rv3), "expecting no k3 to have been stored")
	rv4 := client.Get(k4)
	require.Equal(t, 0, len(rv4), "expecting no k4 to have been stored")
	bat.Write()
	rv3 = client.Get(k3)
	require.Equal(t, rv3, v3, "expecting k3 to have been stored")
	rv4 = client.Get(k4)
	require.Equal(t, rv4, v4, "expecting k4 to have been stored")

	// Batch tests - deletion
	bat = client.NewBatch()
	bat.Delete(k4)
	bat.Delete(k3)
	bat.WriteSync()
	rv3 = client.Get(k3)
	require.Equal(t, 0, len(rv3), "expecting k3 to have been deleted")
	rv4 = client.Get(k4)
	require.Equal(t, 0, len(rv4), "expecting k4 to have been deleted")

	// Batch tests - set and delete
	bat = client.NewBatch()
	bat.Set(k4, v4)
	bat.Set(k5, v5)
	bat.Delete(k4)
	bat.WriteSync()
	rv4 = client.Get(k4)
	require.Equal(t, 0, len(rv4), "expecting k4 to have been deleted")
	rv5 := client.Get(k5)
	require.Equal(t, rv5, v5, "expecting k5 to have been stored")
}
