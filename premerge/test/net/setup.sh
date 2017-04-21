#! /bin/bash
set -eu

# grab glide for dependency mgmt
go get github.com/Masterminds/glide

# grab network monitor, install mintnet, netmon
# these might err 
echo "... fetching repos. ignore go get errors"
set +e
go get github.com/tendermint/network_testing
go get github.com/tendermint/mintnet
go get github.com/tendermint/netmon
set -e

# install vendored deps
echo "GOPATH $GOPATH"

cd $GOPATH/src/github.com/tendermint/mintnet
echo "... install mintnet dir $(pwd)"
glide install
go install
cd $GOPATH/src/github.com/tendermint/netmon
echo "... install netmon dir $(pwd)"
glide install
go install
