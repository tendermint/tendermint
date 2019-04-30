# 1 Guide Assumptions

This guide is designed for beginners who want to get started with a Tendermint
Core application from scratch. It does not assume that you have any prior
experience with Tendermint Core.

Tendermint Core is Byzantine Fault Tolerant (BFT) middleware that takes a state
transition machine - written in any programming language - and securely
replicates it on many machines.

Although Tendermint Core is written in the Golang programming language, prior
knowledge of it is not required for this guide. You can learn it as we go due
to it's simplicity. However, you may want to go through [Learn X in Y minutes
Where X=Go](https://learnxinyminutes.com/docs/go/) first to familiarize
yourself with the syntax.

By following along with this guide, you'll create a Tendermint Core project
called kvstore, a (very) simple distributed key-value store.

# 1 Creating a built-in application in Go

Running your application inside the same process as Tendermint Core will give
you the best possible performance.

For other languages, your application have to communicate with Tendermint Core
through a TCP or Unix domain socket (gRPC option is also supported).

## 1.1 Installing Go

Please refer to [the official guide for installing
Go](https://golang.org/doc/install).

Verify that you have a current version of Go installed:

```sh
$ go version
go version go1.12.4 darwin/amd64
```

Make sure you have `$GOPATH` environment variable set:

```sh
$ echo $GOPATH
/Users/melekes/go
```

## 1.2 Creating a new Go project

We'll start by creating a new Go project.

```sh
$ mkdir -p $GOPATH/src/github.com/me/kvstore
$ cd $GOPATH/src/github.com/me/kvstore
```

Inside the example directory create a `main.go` file with the following content:

```go
package main

import (
	"fmt"
)

func main() {
	fmt.Println("Hello, Tendermint Core")
}
```

When run, this should print "Hello, Tendermint Core" to the standard output.

```sh
$ go run main.go
Hello, Tendermint Core
```

## 1.3 Writing a Tendermint Core application

Tendermint Core communicates with the application through the Application
BlockChain Interface (ABCI). All message types are defined in a [protobuf
file](https://github.com/tendermint/tendermint/blob/develop/abci/types/types.proto).
This allows Tendermint to run applications written in any programming language.

```go
type KVStoreApplication struct {
}

var _ abcitypes.Application = (*KVStoreApplication)(nil)

func NewBaseApplication() *KVStoreApplication {
	return &KVStoreApplication{}
}

func (KVStoreApplication) Info(req RequestInfo) ResponseInfo {
	return ResponseInfo{}
}

func (KVStoreApplication) SetOption(req RequestSetOption) ResponseSetOption {
	return ResponseSetOption{}
}

func (KVStoreApplication) DeliverTx(tx []byte) ResponseDeliverTx {
	return ResponseDeliverTx{Code: CodeTypeOK}
}

func (KVStoreApplication) CheckTx(tx []byte) ResponseCheckTx {
	return ResponseCheckTx{Code: CodeTypeOK}
}

func (KVStoreApplication) Commit() ResponseCommit {
	return ResponseCommit{}
}

func (KVStoreApplication) Query(req RequestQuery) ResponseQuery {
	return ResponseQuery{Code: CodeTypeOK}
}

func (KVStoreApplication) InitChain(req RequestInitChain) ResponseInitChain {
	return ResponseInitChain{}
}

func (KVStoreApplication) BeginBlock(req RequestBeginBlock) ResponseBeginBlock {
	return ResponseBeginBlock{}
}

func (KVStoreApplication) EndBlock(req RequestEndBlock) ResponseEndBlock {
	return ResponseEndBlock{}
}
```

Let's go through each method.

### 1.3.1 CheckTx

When a new transaction is added to the Tendermint Core, it will ask the
application to check it.

For now, we'll make all the transactions valid by responding with `0` code.

```go
func (app *KVStoreApplication) CheckTx(tx []byte) types.ResponseCheckTx {
	return types.ResponseCheckTx{Code: 0, GasWanted: 1}
}
```

TODO: explain gas? :(

### 1.3.2 BeginBlock -> DeliverTx -> EndBlock

```go
func (app *KVStoreApplication) DeliverTx(tx []byte) types.ResponseDeliverTx {
	var key, value []byte
	parts := bytes.Split(tx, []byte("="))
	if len(parts) == 2 {
		key, value = parts[0], parts[1]
	} else {
		key, value = tx, tx
	}
	app.state.db.Set(prefixKey(key), value)
	app.state.Size += 1

	tags := []cmn.KVPair{
		{Key: []byte("app.creator"), Value: []byte("Cosmoshi Netowoko")},
		{Key: []byte("app.key"), Value: key},
	}
	return types.ResponseDeliverTx{Code: code.CodeTypeOK, Tags: tags}
}
```

### 1.3.3 Commit

```go
func (app *KVStoreApplication) Commit() types.ResponseCommit {
	// Using a memdb - just return the big endian size of the db
	appHash := make([]byte, 8)
	binary.PutVarint(appHash, app.state.Size)
	app.state.AppHash = appHash
	app.state.Height += 1
	saveState(app.state)
	return types.ResponseCommit{Data: appHash}
}
```

### 1.3.4 InitChain

```go
func (app *KVStoreApplication) InitChain(req types.RequestInitChain) types.ResponseInitChain {
	return types.ResponseInitChain{}
}
```

### 1.3.5 Info & Query & SetOption

```go
func (app *KVStoreApplication) Info(req types.RequestInfo) (resInfo types.ResponseInfo) {
	return types.ResponseInfo{
		Data:       fmt.Sprintf("{\"size\":%v}", app.state.Size),
		Version:    version.ABCIVersion,
		AppVersion: ProtocolVersion.Uint64(),
	}
}
```

```go
func (app *KVStoreApplication) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	if reqQuery.Prove {
		value := app.state.db.Get(prefixKey(reqQuery.Data))
		resQuery.Index = -1 // TODO make Proof return index
		resQuery.Key = reqQuery.Data
		resQuery.Value = value
		if value != nil {
			resQuery.Log = "exists"
		} else {
			resQuery.Log = "does not exist"
		}
		return
	} else {
		resQuery.Key = reqQuery.Data
		value := app.state.db.Get(prefixKey(reqQuery.Data))
		resQuery.Value = value
		if value != nil {
			resQuery.Log = "exists"
		} else {
			resQuery.Log = "does not exist"
		}
		return
	}
}
```

The complete specification can be found
[here](https://tendermint.com/docs/spec/abci/).

## 1.4 Starting an application and a Tendermint Core instance in the same process

```go
func main() {
	app := kvstore.NewKVStoreApplication()
	node = rpctest.StartTendermint(app)
	// and shut down proper at the end
	rpctest.StopTendermint(node)
}

func StartTendermint(app abci.Application, opts ...func(*Options)) *nm.Node {
	nodeOpts := defaultOptions
	for _, opt := range opts {
		opt(&nodeOpts)
	}
	node := NewTendermint(app, &nodeOpts)
	err := node.Start()
	if err != nil {
		panic(err)
	}

	// wait for rpc
	waitForRPC()
	waitForGRPC()

	if !nodeOpts.suppressStdout {
		fmt.Println("Tendermint running!")
	}

	return node
}

func NewTendermint(app abci.Application, opts *Options) *nm.Node {
	// Create & start node
	config := GetConfig(opts.recreateConfig)
	var logger log.Logger
	if opts.suppressStdout {
		logger = log.NewNopLogger()
	} else {
		logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))
		logger = log.NewFilter(logger, log.AllowError())
	}
	pvKeyFile := config.PrivValidatorKeyFile()
	pvKeyStateFile := config.PrivValidatorStateFile()
	pv := privval.LoadOrGenFilePV(pvKeyFile, pvKeyStateFile)
	papp := proxy.NewLocalClientCreator(app)
	nodeKey, err := p2p.LoadOrGenNodeKey(config.NodeKeyFile())
	if err != nil {
		panic(err)
	}
	node, err := nm.NewNode(config, pv, nodeKey, papp,
		state.DefaultGenesisDocProviderFunc(config),
		nm.DefaultDBProvider,
		nm.DefaultMetricsProvider(config.Instrumentation),
		logger)
	if err != nil {
		panic(err)
	}
	return node
}

// StopTendermint stops a test tendermint server, waits until it's stopped and
// cleans up test/config files.
func StopTendermint(node *nm.Node) {
	node.Stop()
	node.Wait()
	os.RemoveAll(node.Config().RootDir)
}
```

## 1.5 Getting Up and Running

```
$ go run main.go
```

Now open another tab in your terminal and try sending a transaction:

```
$
```
