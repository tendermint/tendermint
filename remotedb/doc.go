/*
remotedb is a package for connecting to distributed Tendermint db.DB
instances. The purpose is to detach difficult deployments such as
CLevelDB that requires gcc or perhaps for databases that require
custom configurations such as extra disk space. It also eases
the burden and cost of deployment of dependencies for databases
to be used by Tendermint developers. Most importantly it is built
over the high performant gRPC transport.

remotedb's RemoteDB implements db.DB so can be used normally
like other databases. One just has to explicitly connect to the
remote database with a client setup such as:

	client, err := remotedb.NewRemoteDB(addr, cert)
	// Make sure to invoke InitRemote!
	if err := client.InitRemote(&remotedb.Init{Name: "test-remote-db", Type: "leveldb"}); err != nil {
	    log.Fatalf("Failed to initialize the remote db")
	}

	client.Set(key1, value)
	gv1 := client.SetSync(k2, v2)

	client.Delete(k1)
	gv2 := client.Get(k1)

	for itr := client.Iterator(k1, k9); itr.Valid(); itr.Next() {
	    ik, iv := itr.Key(), itr.Value()
	    ds, de := itr.Domain()
	}

	stats := client.Stats()

	if !client.Has(dk1) {
	      client.SetSync(dk1, dv1)
	}
*/
package remotedb
