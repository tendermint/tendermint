NOTE: this wiki is mostly deprecated and left for archival purposes. Please see the [documentation website](http://tendermint.readthedocs.io/en/master/) which is built from the [docs directory](https://github.com/tendermint/tendermint/tree/master/docs). Additional information about the specification can also be found in that directory.

# Install Go

Make sure you have installed Go and [set the GOPATH](https://github.com/tendermint/tendermint/wiki/Setting-GOPATH).

# Install Tendermint

You should be able to install the latest with a simple `go get -u github.com/tendermint/tendermint/cmd/tendermint`.
The `-u` makes sure all dependencies are updated as well. 

See `tendermint version` and `tendermint --help` for more.

## Vendored dependencies

If updated dependencies break builds, install the vendored commits using `glide`.

First, install `glide`:

```
go get github.com/Masterminds/glide
```

Now, fetch the dependencies and install them with `glide` and `go`:

```
cd $GOPATH/src/github.com/tendermint/tendermint
glide install
go install ./cmd/tendermint
```

The latest Tendermint Core version is now installed. Check by running `tendermint version`.

## Troubleshooting

If `go get` failing bothers you, fetch the code using `git`:

```
mkdir -p $GOPATH/src/github.com/tendermint
git clone https://github.com/tendermint/tendermint $GOPATH/src/github.com/tendermint/tendermint
```

# Run

To start a one-node blockchain with a simple in-process application: 

```
tendermint init
tendermint node --proxy_app=dummy
```

See the [application developers guide](https://github.com/tendermint/tendermint/wiki/Application-Developers) for more details on building and running applications.
