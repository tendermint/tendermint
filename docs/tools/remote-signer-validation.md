# tm-signer-harness

Located under the `tools/tm-signer-harness` folder in the [Tendermint
repository](https://github.com/tendermint/tendermint).

The Tendermint remote signer test harness facilitates integration testing
between Tendermint and remote signers such as
[KMS](https://github.com/tendermint/kms). Such remote signers allow for signing
of important Tendermint messages using
[HSMs](https://en.wikipedia.org/wiki/Hardware_security_module), providing
additional security.

When executed, `tm-signer-harness`:

1. Runs a listener (either TCP or Unix sockets).
2. Waits for a connection from the remote signer.
3. Upon connection from the remote signer, executes a number of automated tests
   to ensure compatibility.
4. Upon successful validation, the harness process exits with a 0 exit code.
   Upon validation failure, it exits with a particular exit code related to the
   error.

## Prerequisites
Requires the same prerequisites as for building
[Tendermint](https://github.com/tendermint/tendermint).

## Building
From the `tools/tm-signer-harness` directory in your Tendermint source
repository, simply run:

```bash
make

# To have global access to this executable
make install
```

## Docker Image
To build a Docker image containing the `tm-signer-harness`, also from the
`tools/tm-signer-harness` directory of your Tendermint source repo, simply run:

```bash
make docker-image
```

## Running against KMS
As an example of how to use `tm-signer-harness`, the following instructions show
you how to execute its tests against [KMS](https://github.com/tendermint/kms).
For this example, we will make use of the **software signing module in KMS**, as
the hardware signing module requires a physical
[YubiHSM](https://www.yubico.com/products/yubihsm/) device.

### Step 1: Install KMS on your local machine
See the [KMS repo](https://github.com/tendermint/kms) for details on how to set
KMS up on your local machine.

If you have [Rust](https://www.rust-lang.org/) installed on your local machine,
you can simply install KMS by:

```bash
cargo install tmkms
```

### Step 2: Make keys for KMS
The KMS software signing module needs a key with which to sign messages. In our
example, we will simply export a signing key from our local Tendermint instance.

```bash
# Will generate all necessary Tendermint configuration files, including:
# - ~/.tendermint/config/priv_validator_key.json
# - ~/.tendermint/data/priv_validator_state.json
tendermint init

# Extract the signing key from our local Tendermint instance
tm-signer-harness extract_key \      # Use the "extract_key" command
    -tmhome ~/.tendermint \          # Where to find the Tendermint home directory
    -output ./signing.key            # Where to write the key
```

Also, because we want KMS to connect to `tm-signer-harness`, we will need to
provide a secret connection key from KMS' side:

```bash
tmkms keygen secret_connection.key
```

### Step 3: Configure and run KMS
KMS needs some configuration to tell it to use the softer signing module as well
as the `signing.key` file we just generated. Save the following to a file called
`tmkms.toml`:

```toml
[[validator]]
addr = "tcp://127.0.0.1:61219"         # This is where we will find tm-signer-harness.
chain_id = "test-chain-0XwP5E"         # The Tendermint chain ID for which KMS will be signing (found in ~/.tendermint/config/genesis.json).
reconnect = true                       # true is the default
secret_key = "./secret_connection.key" # Where to find our secret connection key.

[[providers.softsign]]
id = "test-chain-0XwP5E"               # The Tendermint chain ID for which KMS will be signing (same as validator.chain_id above).
path = "./signing.key"                 # The signing key we extracted earlier.
```

Then run KMS with this configuration:

```bash
tmkms start -c tmkms.toml
```

This will start KMS, which will repeatedly try to connect to
`tcp://127.0.0.1:61219` until it is successful.

### Step 4: Run tm-signer-harness
Now we get to run the signer test harness:

```bash
tm-signer-harness run \             # The "run" command executes the tests
    -addr tcp://127.0.0.1:61219 \   # The address we promised KMS earlier
    -tmhome ~/.tendermint           # Where to find our Tendermint configuration/data files.
```

If the current version of Tendermint and KMS are compatible, `tm-signer-harness`
should now exit with a 0 exit code. If they are somehow not compatible, it
should exit with a meaningful non-zero exit code (see the exit codes below).

### Step 5: Shut down KMS
Simply hit Ctrl+Break on your KMS instance (or use the `kill` command in Linux)
to terminate it gracefully.

## Exit Code Meanings
The following list shows the various exit codes from `tm-signer-harness` and
their meanings:

| Exit Code | Description |
| --- | --- |
| 0 | Success! |
| 1 | Invalid command line parameters supplied to `tm-signer-harness` |
| 2 | Maximum number of accept retries reached (the `-accept-retries` parameter) |
| 3 | Failed to load `${TMHOME}/config/genesis.json` |
| 4 | Failed to create listener specified by `-addr` parameter |
| 5 | Failed to start listener |
| 6 | Interrupted by `SIGINT` (e.g. when hitting Ctrl+Break or Ctrl+C) |
| 7 | Other unknown error |
| 8 | Test 1 failed: public key mismatch |
| 9 | Test 2 failed: signing of proposals failed |
| 10 | Test 3 failed: signing of votes failed |
