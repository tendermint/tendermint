package main

import (
	acm "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	btypes "github.com/tendermint/tendermint/cmd/barak/types"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/rpc"
)

// Convenience function for a single validator.
func ListProcesses(privKey acm.PrivKey, remote string) (btypes.ResponseListProcesses, error) {
	command := btypes.CommandListProcesses{}
	nonce := GetNonce(remote)
	commandBytes, signature := SignCommand(privKey, nonce+1, command)
	response := btypes.ResponseListProcesses{}
	_, err := RunAuthCommand(remote, commandBytes, []acm.Signature{signature}, &response)
	return response, err
}

//-----------------------------------------------------------------------------

// Utility method to get nonce from the remote.
// The next command should include the returned nonce+1 as nonce.
func GetNonce(remote string) uint64 {
	var err error
	response := btypes.ResponseStatus{}
	_, err = rpc.Call(remote, "status", Arr(), &response)
	if err != nil {
		panic(Fmt("Error fetching nonce from remote %v: %v", remote, err))
	}
	return response.Nonce
}

// Each developer runs this
func SignCommand(privKey acm.PrivKey, nonce uint64, command btypes.Command) ([]byte, acm.Signature) {
	noncedCommand := btypes.NoncedCommand{
		Nonce:   nonce,
		Command: command,
	}
	commandJSONBytes := binary.JSONBytes(noncedCommand)
	signature := privKey.Sign(commandJSONBytes)
	return commandJSONBytes, signature
}

// Somebody aggregates the signatures and calls this.
func RunAuthCommand(remote string, commandJSONBytes []byte, signatures []acm.Signature, dest interface{}) (interface{}, error) {
	authCommand := btypes.AuthCommand{
		CommandJSONStr: string(commandJSONBytes),
		Signatures:     signatures,
	}
	return rpc.Call(remote, "run", Arr(authCommand), dest)
}
