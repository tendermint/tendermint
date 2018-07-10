# KVStore

There are two app's here: the KVStoreApplication and the PersistentKVStoreApplication.

## KVStoreApplication

The KVStoreApplication is a simple merkle key-value store. 
Transactions of the form `key=value` are stored as key-value pairs in the tree.
Transactions without an `=` sign set the value to the key.
The app has no replay protection (other than what the mempool provides).

## PersistentKVStoreApplication

The PersistentKVStoreApplication wraps the KVStoreApplication
and provides two additional features:

1) persistence of state across app restarts (using Tendermint's ABCI-Handshake mechanism)
2) validator set changes

The state is persisted in leveldb along with the last block committed,
and the Handshake allows any necessary blocks to be replayed.
Validator set changes are effected using the following transaction format:

```
val:pubkey1/power1,addr2/power2,addr3/power3"
```

where `power1` is the new voting power for the validator with `pubkey1` (possibly a new one).
There is no sybil protection against new validators joining. 
Validators can be removed by setting their power to `0`.

