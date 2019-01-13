/*

Package privval provides different implementations of the types.PrivValidator.

FilePV

FilePV is the simplest implementation and developer default. It uses one file for the private key and another to store state.

SocketVal

SocketVal establishes a connection to an external process, like a Key Management Server (KMS), using a socket.
SocketVal listens for the external KMS process to dial in.
SocketVal takes a listener, which determines the type of connection
(ie. encrypted over tcp, or unencrypted over unix).

RemoteSigner

RemoteSigner is a simple wrapper around a net.Conn. It's used by both IPCVal and TCPVal.

*/
package privval
