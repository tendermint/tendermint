Simulate the Economy of Texas.

```bash
sim_txs --priv-key "PRIV_KEY_HEX" --num-accounts 1000 --remote localhost:46657
```

The above uses the rpc-host to fetch the account state for the account with given priv-key,
then deterministically generates num-accounts more accounts to spread the coins around.

It's a test utility.
