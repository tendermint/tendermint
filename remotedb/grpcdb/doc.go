/*
grpcdb is the distribution of Tendermint's db.DB instances using
the gRPC transport to decouple local db.DB usages from applications,
to using them over a network in a highly performant manner.

grpcdb allows users to initialize a database's server like
they would locally and invoke the respective methods of db.DB.

Most users shouldn't use this package, but should instead use
remotedb. Only the lower level users and database server deployers
should use it, for functionality such as:

	ln, err := net.Listen("tcp", "0.0.0.0:0")
	srv := grpcdb.NewServer()
	defer srv.Stop()
	go func() {
		if err := srv.Serve(ln); err != nil {
			t.Fatalf("BindServer: %v", err)
		}
	}()

or
	addr := ":8998"
	cert := "server.crt"
	key := "server.key"
	go func() {
		if err := grpcdb.ListenAndServe(addr, cert, key); err != nil {
			log.Fatalf("BindServer: %v", err)
		}
	}()
*/
package grpcdb
