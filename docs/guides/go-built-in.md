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

Create a file called `app.go` with the following content:

```go
package main

import (
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

type KVStoreApplication struct {
}

var _ abcitypes.Application = (*KVStoreApplication)(nil)

func NewKVStoreApplication() *KVStoreApplication {
	return &KVStoreApplication{}
}

func (KVStoreApplication) Info(req abcitypes.RequestInfo) abcitypes.ResponseInfo {
	return abcitypes.ResponseInfo{}
}

func (KVStoreApplication) SetOption(req abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
	return abcitypes.ResponseSetOption{}
}

func (KVStoreApplication) DeliverTx(tx []byte) abcitypes.ResponseDeliverTx {
	return abcitypes.ResponseDeliverTx{Code: 0}
}

func (KVStoreApplication) CheckTx(tx []byte) abcitypes.ResponseCheckTx {
	return abcitypes.ResponseCheckTx{Code: 0}
}

func (KVStoreApplication) Commit() abcitypes.ResponseCommit {
	return abcitypes.ResponseCommit{}
}

func (KVStoreApplication) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{Code: 0}
}

func (KVStoreApplication) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
	return abcitypes.ResponseInitChain{}
}

func (KVStoreApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	return abcitypes.ResponseBeginBlock{}
}

func (KVStoreApplication) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	return abcitypes.ResponseEndBlock{}
}
```

Now I will go through each method explaining when it's called and adding
required business logic.

### 1.3.1 CheckTx

When a new transaction is added to the Tendermint Core, it will ask the
application to check it.

For transactions, which have a form of `{bytes}={bytes}`, we'll return a zero
code indicating that they are valid. For others, we'll respond with `1`. Note
that anything with non-zero code will be considered invalid (`-1`, `100`,
etc.) by Tendermint Core.

```go
func (app *KVStoreApplication) CheckTx(tx []byte) abcitypes.ResponseCheckTx {
  // check format
	parts := bytes.Split(tx, []byte("="))
	if len(parts) != 2 {
		return abcitypes.ResponseCheckTx{Code: 1, GasWanted: 1}
	}

	key, value := parts[0], parts[1]
	code := 0 // OK

  // check if the same key=value already exists
	err := db.View(func(txn *badger.Txn) error {
		existing, err := txn.Get(key)
		if err != badger.ErrKeyNotFound && existing == value {
			code = 2
		}
		return nil
	})

	return abcitypes.ResponseCheckTx{Code: code, GasWanted: 1}
}
```

For the actual underlying key-value store we'll use
[badger](https://github.com/dgraph-io/badger), which is an embeddable,
persistent and fast key-value (KV) database.

```go
import "github.com/dgraph-io/badger"

type KVStoreApplication struct {
	db *badger.DB
}

var _ abcitypes.Application = (*KVStoreApplication)(nil)

func NewKVStoreApplication(db *badger.DB) *KVStoreApplication {
	return &KVStoreApplication{
		db: db,
	}
}
```

TODO: explain gas? :(

### 1.3.2 BeginBlock -> DeliverTx -> EndBlock -> Commit

When Tendermint Core has decided on the block, it's transfered to the
application in 3 parts: `BeginBlock`, one `DeliverTx` per transaction and
`EndBlock` in the end.

For this guide, we'll only need `DeliverTx` and `Commit`.

```go
func (app *KVStoreApplication) DeliverTx(tx []byte) types.ResponseDeliverTx {
	parts := bytes.Split(tx, []byte("="))
	if len(parts) != 2 {
	  return types.ResponseDeliverTx{Code: 1}
	}
	key, value := parts[0], parts[1]
  existing, ok := app.db.Get(key)
  if ok {
    if existing == value { // already exists
	    return types.ResponseDeliverTx{Code: 2}
    }
	  app.currentBatch.Set(key, value)
    app.updated++
  } else {
    app.new++
  }
	return types.ResponseDeliverTx{Code: 0}
}
```

`Commit` instructs the application to persist the new state.
TODO: explain why we need appHash

```go
func (app *KVStoreApplication) Commit() types.ResponseCommit {
  app.currentBatch.Save()

	app.state.Height++
	app.state.New += app.new
	app.state.Updated += app.updated
  app.new = 0
  app.updated = 0
  app.state.Save()

	return types.ResponseCommit{Data: app.Hash()}
}
```

### 1.3.3 InitChain

```go
func (app *KVStoreApplication) InitChain(req types.RequestInitChain) types.ResponseInitChain {
	return types.ResponseInitChain{}
}
```

### 1.3.4 Query

```go
func (app *KVStoreApplication) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
  resQuery.Key = reqQuery.Data
  value := app.db.Get(reqQuery.Data)
  resQuery.Value = value
  if value != nil {
    resQuery.Log = "exists"
  } else {
    resQuery.Log = "does not exist"
  }
  return
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
