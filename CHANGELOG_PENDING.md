# Unreleased Changes

## v0.34.25

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES
[rpc] [#9996](https://github.com/tendermint/tendermint/issues/9996) Add config option to run RPC endpoint with mTLS support:
```
# The path to a file containing CA certificate who issues client's certificates for mutual TLS (mTLS).
# Might be either absolute path or path related to tendermint's config directory.
# Note: in case of empty value - mutual TLS is disabled (default).
tls_client_cacert_file = ""
```
### IMPROVEMENTS

### BUG FIXES

