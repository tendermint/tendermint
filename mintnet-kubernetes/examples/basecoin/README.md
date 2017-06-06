# Basecoin example

This is an example of using [basecoin](https://github.com/tendermint/basecoin).

## Usage

```
make create
```

### Check account balance and send a transaction

1. wait until all the pods are `Running`.

   ```
   kubectl get pods -w -o wide -L tm
   ```

2. wait until app starts.

   ```
   kubectl logs -c app -f tm-0
   ```

3. get account's address of the second pod

   ```
   ADDR=`kubectl exec -c app tm-1 -- cat /app/key.json | jq ".address" | tr -d "\""`
   ```

4. send 5 coins to it from the first pod

   ```
   kubectl exec -c app tm-0 -- basecoin tx send --to "0x$ADDR" --amount 5mycoin --from /app/key.json --chain_id chain-tTH4mi
   ```


## Clean up

```
make destroy
```
