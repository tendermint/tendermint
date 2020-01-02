package remotedb_test

import (
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tm-db/remotedb"
	"github.com/tendermint/tm-db/remotedb/grpcdb"
)

func TestRemoteDB(t *testing.T) {
	cert := "test.crt"
	key := "test.key"
	ln, err := net.Listen("tcp", "localhost:0")
	require.Nil(t, err, "expecting a port to have been assigned on which we can listen")
	srv, err := grpcdb.NewServer(cert, key)
	require.Nil(t, err)
	defer srv.Stop()
	go func() { //nolint:staticcheck
		if err := srv.Serve(ln); err != nil {
			t.Fatalf("BindServer: %v", err)
		}
	}()

	client, err := remotedb.NewRemoteDB(ln.Addr().String(), cert)
	require.Nil(t, err, "expecting a successful client creation")
	dbName := "test-remote-db"
	require.Nil(t, client.InitRemote(&remotedb.Init{Name: dbName, Type: "goleveldb"}))
	defer func() {
		err := os.RemoveAll(dbName + ".db")
		if err != nil {
			panic(err)
		}
	}()

	k1 := []byte("key-1")
	v1, err := client.Get(k1)
	require.NoError(t, err)
	require.Equal(t, 0, len(v1), "expecting no key1 to have been stored, got %X (%s)", v1, v1)
	vv1 := []byte("value-1")
	err = client.Set(k1, vv1)
	require.NoError(t, err)

	gv1, err := client.Get(k1)
	require.NoError(t, err)
	require.Equal(t, gv1, vv1)

	// Simple iteration
	itr, err := client.Iterator(nil, nil)
	require.NoError(t, err)

	itr.Next()

	key1 := itr.Key()
	value := itr.Value()

	require.Equal(t, key1, []byte("key-1"))
	require.Equal(t, value, []byte("value-1"))
	itr.Close()

	// Set some more keys
	k2 := []byte("key-2")
	v2 := []byte("value-2")
	err = client.SetSync(k2, v2)
	require.NoError(t, err)
	has, err := client.Has(k2)
	require.NoError(t, err)
	require.True(t, has)
	gv2, err := client.Get(k2)
	require.NoError(t, err)
	require.Equal(t, gv2, v2)

	// More iteration
	itr, err = client.Iterator(nil, nil)
	require.NoError(t, err)

	itr.Next()

	key1 = itr.Key()

	value = itr.Value()

	require.Equal(t, key1, []byte("key-1"))
	require.Equal(t, value, []byte("value-1"))
	itr.Next()

	key1 = itr.Key()

	value = itr.Value()
	require.Equal(t, key1, []byte("key-2"))
	require.Equal(t, value, []byte("value-2"))
	itr.Close()

	// Deletion
	err = client.Delete(k1)
	require.NoError(t, err)
	err = client.DeleteSync(k2)
	require.NoError(t, err)
	gv1, err = client.Get(k1)
	require.NoError(t, err)
	gv2, err = client.Get(k2)
	require.NoError(t, err)
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

	rv3, err := client.Get(k3)
	require.NoError(t, err)
	require.Equal(t, 0, len(rv3), "expecting no k3 to have been stored")

	rv4, err := client.Get(k4)
	require.NoError(t, err)
	require.Equal(t, 0, len(rv4), "expecting no k4 to have been stored")
	err = bat.Write()
	require.NoError(t, err)

	rv3, err = client.Get(k3)
	require.NoError(t, err)
	require.Equal(t, rv3, v3, "expecting k3 to have been stored")

	rv4, err = client.Get(k4)
	require.NoError(t, err)
	require.Equal(t, rv4, v4, "expecting k4 to have been stored")

	// Batch tests - deletion
	bat = client.NewBatch()
	bat.Delete(k4)
	bat.Delete(k3)
	err = bat.WriteSync()
	require.NoError(t, err)

	rv3, err = client.Get(k3)
	require.NoError(t, err)
	require.Equal(t, 0, len(rv3), "expecting k3 to have been deleted")

	rv4, err = client.Get(k4)
	require.NoError(t, err)
	require.Equal(t, 0, len(rv4), "expecting k4 to have been deleted")

	// Batch tests - set and delete
	bat = client.NewBatch()
	bat.Set(k4, v4)
	bat.Set(k5, v5)
	bat.Delete(k4)
	err = bat.WriteSync()
	require.NoError(t, err)

	rv4, err = client.Get(k4)
	require.NoError(t, err)
	require.Equal(t, 0, len(rv4), "expecting k4 to have been deleted")

	rv5, err := client.Get(k5)
	require.NoError(t, err)
	require.Equal(t, rv5, v5, "expecting k5 to have been stored")
}
