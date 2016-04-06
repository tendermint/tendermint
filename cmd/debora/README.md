### Example

```bash
# Upgrade barak.
# We need to give it a new seed to prevent port conflicts.
./build/debora run --input "`cat cmd/barak/seed2`" -- barak2 ./build/barak

# Build tendermint from source
./build/debora run -- build_tendermint bash -c "cd $GOPATH/src/github.com/eris-ltd/tendermint; make"

# Build and run tendermint
./build/debora run -- tendermint bash -c "cd \$GOPATH/src/github.com/eris-ltd/tendermint; make; ./build/tendermint node"
```
