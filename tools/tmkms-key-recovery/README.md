# Cosmos Validator Key Recovery

## Description 

Scripts in this directory allow for the validator private key recovery from the seed words, which is useful in case of migration from the ledger nano S to the softsign or YoubiHSM 

> Prerequisites (Python)

```
pip install ECPy
pip install hmac
```

> Execution

Replace `mnemonic` variable in the `tmkms-cosmos.py` script with your seed words, then execute following command

```
python3 ./tmkms-cosmos.py
```

> Output

The output of the command is a 32B Base64 encoded private .key that does not require `tmkms import` and can be used to sing blocks with the KMS

