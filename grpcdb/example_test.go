package grpcdb_test

import (
	"bytes"
	"context"
	"log"

	grpcdb "github.com/tendermint/tmlibs/grpcdb"
	protodb "github.com/tendermint/tmlibs/proto"
)

func Example() {
	addr := ":8998"
	go func() {
		if err := grpcdb.BindServer(addr); err != nil {
			log.Fatalf("BindServer: %v", err)
		}
	}()

	client, err := grpcdb.NewClient(addr, grpcdb.Insecure)
	if err != nil {
		log.Fatalf("Failed to create grpcDB client: %v", err)
	}

	ctx := context.Background()
	// 1. Initialize the DB
	in := &protodb.Init{
		Type: "leveldb",
		Name: "grpc-uno-test",
		Dir:  ".",
	}
	if _, err := client.Init(ctx, in); err != nil {
		log.Fatalf("Init error: %v", err)
	}

	// 2. Now it can be used!
	query1 := &protodb.Entity{Key: []byte("Project"), Value: []byte("Tmlibs-on-gRPC")}
	if _, err := client.SetSync(ctx, query1); err != nil {
		log.Fatalf("SetSync err: %v", err)
	}

	query2 := &protodb.Entity{Key: []byte("Project")}
	read, err := client.Get(ctx, query2)
	if err != nil {
		log.Fatalf("Get err: %v", err)
	}
	if g, w := read.Value, []byte("Tmlibs-on-gRPC"); !bytes.Equal(g, w) {
		log.Fatalf("got= (%q ==> % X)\nwant=(%q ==> % X)", g, g, w, w)
	}
}
