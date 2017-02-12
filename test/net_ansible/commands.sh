go get -u github.com/tendermint/tendermint/cmd/tendermint

cd $GOPATH/src/github.com/tendermint/tendermint

# use branch for the code
rm -rf $GOPATH/src/github.com/tendermint/testmint
mkdir -p $GOPATH/src/github.com/tendermint/testmint
git clone https://github.com/tendermint/tendermint $GOPATH/src/github.com/tendermint/tendermint
cd $GOPATH/src/github.com/tendermint/tendermint
git checkout testnet

cd $GOPATH/src/github.com/tendermint/tendermint/test/net_ansible

# create testnet files
tendermint testnet 4 mytestnet


terraform get
terraform apply
