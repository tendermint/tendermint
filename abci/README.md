# Application BlockChain Interface (ABCI)

[![CircleCI](https://circleci.com/gh/tendermint/abci.svg?style=svg)](https://circleci.com/gh/tendermint/abci)

Blockchains are systems for multi-master state machine replication.
**ABCI** is an interface that defines the boundary between the replication engine (the blockchain),
and the state machine (the application).
Using a socket protocol, a consensus engine running in one process
can manage an application state running in another.

Previously, the ABCI was referred to as TMSP.

The community has provided a number of addtional implementations, see the [Tendermint Ecosystem](https://tendermint.com/ecosystem)

## Specification

A detailed description of the ABCI methods and message types is contained in:

- [A prose specification](specification.md)
- [A protobuf file](https://github.com/tendermint/abci/blob/master/types/types.proto)
- [A Go interface](https://github.com/tendermint/abci/blob/master/types/application.go).

For more background information on ABCI, motivations, and tendermint, please visit [the documentation](http://tendermint.readthedocs.io/en/master/).
The two guides to focus on are the `Application Development Guide` and `Using ABCI-CLI`.


## Protocl Buffers

To compile the protobuf file, run:

```
make protoc
```

See `protoc --help` and [the Protocol Buffers site](https://developers.google.com/protocol-buffers)
for details on compiling for other languages. Note we also include a [GRPC](http://www.grpc.io/docs)
service definition.

## Install ABCI-CLI

The `abci-cli` is a simple tool for debugging ABCI servers and running some
example apps. To install it:

```
go get github.com/tendermint/abci
cd $GOPATH/src/github.com/tendermint/abci
make get_vendor_deps
make install
```

## Implementation

We provide three implementations of the ABCI in Go:

- Golang in-process
- ABCI-socket
- GRPC

Note the GRPC version is maintained primarily to simplify onboarding and prototyping and is not receiving the same
attention to security and performance as the others

### In Process

The simplest implementation just uses function calls within Go.
This means ABCI applications written in Golang can be compiled with TendermintCore and run as a single binary.

See the [examples](#examples) below for more information.

### Socket (TSP)

ABCI is best implemented as a streaming protocol.
The socket implementation provides for asynchronous, ordered message passing over unix or tcp.
Messages are serialized using Protobuf3 and length-prefixed with a [signed Varint](https://developers.google.com/protocol-buffers/docs/encoding?csw=1#signed-integers)

For example, if the Protobuf3 encoded ABCI message is `0xDEADBEEF` (4 bytes), the length-prefixed message is `0x08DEADBEEF`, since `0x08` is the signed varint
encoding of `4`. If the Protobuf3 encoded ABCI message is 65535 bytes long, the length-prefixed message would be like `0xFEFF07...`.

Note the benefit of using this `varint` encoding over the old version (where integers were encoded as `<len of len><big endian len>` is that
it is the standard way to encode integers in Protobuf. It is also generally shorter.

### GRPC

GRPC is an rpc framework native to Protocol Buffers with support in many languages.
Implementing the ABCI using GRPC can allow for faster prototyping, but is expected to be much slower than
the ordered, asynchronous socket protocol. The implementation has also not received as much testing or review.

Note the length-prefixing used in the socket implementation does not apply for GRPC.

## Usage

The `abci-cli` tool wraps an ABCI client and can be used for probing/testing an ABCI server.
For instance, `abci-cli test` will run a test sequence against a listening server running the Counter application (see below).
It can also be used to run some example applications.
See [the documentation](http://tendermint.readthedocs.io/en/master/) for more details.

### Examples

Check out the variety of example applications in the [example directory](example/).
It also contains the code refered to by the `counter` and `kvstore` apps; these apps come
built into the `abci-cli` binary.

#### Counter

The `abci-cli counter` application illustrates nonce checking in transactions. It's code looks like:

```golang
func cmdCounter(cmd *cobra.Command, args []string) error {

	app := counter.NewCounterApplication(flagSerial)

	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))

	// Start the listener
	srv, err := server.NewServer(flagAddrC, flagAbci, app)
	if err != nil {
		return err
	}
	srv.SetLogger(logger.With("module", "abci-server"))
	if err := srv.Start(); err != nil {
		return err
	}

	// Wait forever
	cmn.TrapSignal(func() {
		// Cleanup
		srv.Stop()
	})
	return nil
}
```

and can be found in [this file](cmd/abci-cli/abci-cli.go).

#### kvstore

The `abci-cli kvstore` application, which illustrates a simple key-value Merkle tree

```golang
func cmdKVStore(cmd *cobra.Command, args []string) error {
	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))

	// Create the application - in memory or persisted to disk
	var app types.Application
	if flagPersist == "" {
		app = kvstore.NewKVStoreApplication()
	} else {
		app = kvstore.NewPersistentKVStoreApplication(flagPersist)
		app.(*kvstore.PersistentKVStoreApplication).SetLogger(logger.With("module", "kvstore"))
	}

	// Start the listener
	srv, err := server.NewServer(flagAddrD, flagAbci, app)
	if err != nil {
		return err
	}
	srv.SetLogger(logger.With("module", "abci-server"))
	if err := srv.Start(); err != nil {
		return err
	}

	// Wait forever
	cmn.TrapSignal(func() {
		// Cleanup
		srv.Stop()
	})
	return nil
}
```
