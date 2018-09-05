# Install Tendermint

The fastest and easiest way to install the `tendermint` binary
is to run [this script](https://github.com/tendermint/tendermint/blob/develop/scripts/install/install_tendermint_ubuntu.sh) on
a fresh Ubuntu instance,
or [this script](https://github.com/tendermint/tendermint/blob/develop/scripts/install/install_tendermint_bsd.sh)
on a fresh FreeBSD instance. Read the comments / instructions carefully (i.e., reset your terminal after running the script,
make sure you are okay with the network connections being made).

## From Binary

To download pre-built binaries, see the [releases page](https://github.com/tendermint/tendermint/releases).

## From Source

You'll need `go` [installed](https://golang.org/doc/install) and the required
[environment variables set](https://github.com/tendermint/tendermint/wiki/Setting-GOPATH)

### Get Source Code

```
mkdir -p $GOPATH/src/github.com/tendermint
cd $GOPATH/src/github.com/tendermint
git clone https://github.com/tendermint/tendermint.git
cd tendermint
```

### Get Tools & Dependencies

```
make get_tools
make get_vendor_deps
```

### Compile

```
make install
```

to put the binary in `$GOPATH/bin` or use:

```
make build
```

to put the binary in `./build`.

The latest `tendermint version` is now installed.

## Run

To start a one-node blockchain with a simple in-process application:

```
tendermint init
tendermint node --proxy_app=kvstore
```

## Reinstall

If you already have Tendermint installed, and you make updates, simply

```
cd $GOPATH/src/github.com/tendermint/tendermint
make install
```

To upgrade, run

```
cd $GOPATH/src/github.com/tendermint/tendermint
git pull origin master
make get_vendor_deps
make install
```

## Compile with CLevelDB support

Make sure you have a roughly compatible version of libstdc++ (tested with
5.3.1). For example, on Ubuntu:

```
sudo apt-get update
sudo apt-get install gcc
sudo apt-cache show libstdc++6
Version: 5.3.1-14ubuntu2
```

Check out leveldb dependency using git (for some reason dep does not check out
submodules):

```
cd vendor/github.com/DataDog && rm -rf leveldb
git clone https://github.com/DataDog/leveldb.git
```

Set database backend to cleveldb:

```
# config/config.toml
db_backend = "cleveldb"
```

To build Tendermint, run

```
CGO_ENABLED=1 CGO_CXXFLAGS_ALLOW="(-fno-builtin-memcmp|-lpthread)" CGO_CFLAGS_ALLOW="-fno-builtin-memcmp" go build -ldflags "-X github.com/tendermint/tendermint/version.GitCommit=`git rev-parse --short=8 HEAD`" -tags "tendermint gcc" -o build/tendermint ./cmd/tendermint/
```
