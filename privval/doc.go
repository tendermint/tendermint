/*
Package privval provides different implementations of the types.PrivValidator.

FilePV

FilePV is the simplest implementation and developer default. It uses one file for the private key and another to store state.

TCPVal

TCPVal uses an encrypted TCP socket to request signatures from an external process, like a Key Management Server (KMS).
Listens for the external KMS process to dial in. Connections are authenticated and encrypted using the p2pconn.SecretConnection.

IPCVal

IPCVal uses an unencrypted unix socket to request signatures from an external process, like a Key Management Server (KMS).
Dials the external process.

NOTE: TCPVal and IPCVal should [be consolidated](https://github.com/tendermint/tendermint/issues/3104).

RemoteSigner

RemoteSigner is a simple wrapper around a net.Conn. It's used by both IPCVal and TCPVal.

*/
package privval
