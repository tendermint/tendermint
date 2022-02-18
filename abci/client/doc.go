// Package abciclient provides an ABCI implementation in Go.
//
// There are 3 clients available:
//		1. socket (unix or TCP)
//		2. local (in memory)
//		3. gRPC
//
// ## Socket client
//
// the client blocks on 1) enqueuing the  request 2) enqueuing the
// Flush requests 3) waiting for the Flush response
//
// ## Local client
//
// global mutex is locked during each call
//
// ## gRPC client
//
// waits for all calls to complete (essentially what Flush does in
// the socket client).
package abciclient
