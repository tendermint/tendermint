// Copyright 2016 Tendermint. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package core_grpc

import (
	"fmt"
	"net"
	"strings"
	"time"

	"google.golang.org/grpc"

	. "github.com/tendermint/tmlibs/common"
)

// Start the grpcServer in a go routine
func StartGRPCServer(protoAddr string) (net.Listener, error) {
	parts := strings.SplitN(protoAddr, "://", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("Invalid listen address for grpc server (did you forget a tcp:// prefix?) : %s", protoAddr)
	}
	proto, addr := parts[0], parts[1]
	ln, err := net.Listen(proto, addr)
	if err != nil {
		return nil, err
	}

	grpcServer := grpc.NewServer()
	RegisterBroadcastAPIServer(grpcServer, &broadcastAPI{})
	go grpcServer.Serve(ln)

	return ln, nil
}

// Start the client by dialing the server
func StartGRPCClient(protoAddr string) BroadcastAPIClient {
	conn, err := grpc.Dial(protoAddr, grpc.WithInsecure(), grpc.WithDialer(dialerFunc))
	if err != nil {
		panic(err)
	}
	return NewBroadcastAPIClient(conn)
}

func dialerFunc(addr string, timeout time.Duration) (net.Conn, error) {
	return Connect(addr)
}
