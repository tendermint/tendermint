# Plasma-mint

testing instructions:
```
cd $GOPATH/src/github.com/
mkdir tendermint
cd tendermint
git clone git@github.com:parsec-labs/tendermint.git
cd tendermint
make get_vendor_deps
```

after this, you should be able to run tests:
```
cd consensus
go test
  PASS
  ok  	github.com/tendermint/tendermint/consensus	11.382s
```
