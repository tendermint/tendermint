// Package abciclient provides an ABCI implementation in Go.
//
// There are 3 clients available:
//		1. socket (unix or TCP)
//		2. local (in memory)
//		3. gRPC
//
// ## Socket client
//
// The client blocks for enqueuing the request, for enqueuing the
// Flush to send the request, and for the Flush response to return.
//
// ## Local client
//
// The global mutex is locked during each call
//
// ## gRPC client
//
// The client waits for all calls to complete.
package abciclient
