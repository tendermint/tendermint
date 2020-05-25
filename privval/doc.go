/*

Package privval provides different implementations of the types.PrivValidator.

FilePV

FilePV is the simplest implementation and developer default.
It uses one file for the private key and another to store state.

Signer Server/Client

Signer Server/Client implements gRPC (https://grpc.io/) for communication on both TCP and UNIX sockets.
The Client is used by the tendermint node to query a server like tmkms (https://github.com/iqlusioninc/tmkms)

If you would like to implment your own server for handling signing you will need to fetch the needed proto directories (proto/privval & proto/types).

*/
package privval
