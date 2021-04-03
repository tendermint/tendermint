// Package abcicli provides an ABCI implementation in Go.
//
// There are 3 clients available:
//		1. socket (unix or TCP)
//		2. local (in memory)
//		3. gRPC
//
// ## Socket client
//
// async: the client maintains an internal buffer of a fixed size. when the
// buffer becomes full, all Async calls will return an error immediately.
//
// sync: the client blocks on 1) enqueuing the Sync request 2) enqueuing the
// Flush requests 3) waiting for the Flush response
//
// ## Local client
//
// async: global mutex is locked during each call (meaning it's not really async!)
// sync: global mutex is locked during each call
//
// ## gRPC client
//
// async: gRPC is synchronous, but an internal buffer of a fixed size is used
// to store responses and later call callbacks (separate goroutine per
// response).
//
// sync: waits for all Async calls to complete (essentially what Flush does in
// the socket client) and calls Sync method.
package abcicli
