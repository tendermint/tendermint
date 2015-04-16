### Example

```bash
# Upgrade barak.
# We need to give it a new seed to prevent port conflicts.
./build/debora --privkey-file build/privkey run --input "`cat cmd/barak/seed2`" -- barak2 ./build/barak

# Build tendermint from source
./build/debora --privkey-file build/privkey run -- build_tendermint bash -c "cd $GOPATH/src/github.com/tendermint/tendermint; make"
```
