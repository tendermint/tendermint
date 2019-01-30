# KVStoreApplication

The KVStoreApplication is a simple merkle key-value store. Transactions of the form key=value are stored as key-value pairs in the tree. Transactions without an = sign set both key and value to the value given. The app has no replay protection (other than what the mempool provides).

# Install

Runs on Python 3.6.7 using [py-abci](https://github.com/davebryson/py-abci) `abci==0.6.0`.

```
pip install -r requirements.txt
```

# RUN

Start the Python application on port `26658`

```
python kvstore.py
```

Start the tendermint node.

```
tendermint init
tendermint node
```

The application doesn't support  PersistentKVStoreApplication yet.