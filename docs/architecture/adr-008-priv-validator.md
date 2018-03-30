# ADR 008: SocketPV

Tendermint node's should support only two in-process PrivValidator
implementations:

- FilePV uses an unencrypted private key in a "priv_validator.json" file - no
  configuration required (just `tendermint init`).
- SocketPV uses a socket to send signing requests to another process - user is
  responsible for starting that process themselves.

The SocketPV address can be provided via flags at the command line - doing so
will cause Tendermint to ignore any "priv_validator.json" file and to listen on
the given address for incoming connections from an external priv_validator
process.  It will halt any operation until at least one external process
succesfully connected.

The external priv_validator process will dial the address to connect to
Tendermint, and then Tendermint will send requests on the ensuing connection to
sign votes and proposals.  Thus the external process initiates the connection,
but the Tendermint process makes all requests.  In a later stage we're going to
support multiple validators for fault tolerance. To prevent double signing they
need to be synced, which is deferred to an external solution (see #1185).

In addition, Tendermint will provide implementations that can be run in that
external process.  These include:

- FilePV will encrypt the private key, and the user must enter password to
  decrypt key when process is started.
- LedgerPV uses a Ledger Nano S to handle all signing.
