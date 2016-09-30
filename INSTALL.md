# Install Go

[Install Go, set the `GOPATH`, and put `GOPATH/bin` on your `PATH`](https://github.com/tendermint/tendermint/wiki/Setting-GOPATH).

# Install Tendermint

You should be able to install the latest with a simple `go get -u github.com/tendermint/tendermint/cmd/tendermint`.
The `-u` makes sure all dependencies are updated as well. 

Run `tendermint version` and `tendermint --help`.

If the install falied, see [vendored dependencies below](#vendored-dependencies).

To start a one-node blockchain with a simple in-process application: 

```
tendermint init
tendermint node --proxy_app=dummy
```

See the [application developers guide](https://github.com/tendermint/tendermint/wiki/Application-Developers) for more details on building and running applications.


## Vendored dependencies

If the `go get` failed, updated dependencies may have broken the build.
Install the correct version of each dependency using `glide`.

Fist, install `glide`:

```
go get github.com/Masterminds/glide
```

Now, fetch the dependencies and install them with `glide` and `go`:

```
cd $GOPATH/src/github.com/tendermint/tendermint
glide install
go install ./cmd/tendermint
```

Sometimes `glide install` is painfully slow. Hang in there champ.

The latest Tendermint Core version is now installed. Check by running `tendermint version`.

## Troubleshooting

If `go get` failing bothers you, fetch the code using `git`:

```
mkdir -p $GOPATH/src/github.com/tendermint
git clone https://github.com/tendermint/tendermint $GOPATH/src/github.com/tendermint/tendermint
cd $GOPATH/src/github.com/tendermint/tendermint
glide install
go install ./cmd/tendermint
```
