package main

import (
	"fmt"
	"io"
	"net/url"
	"os"

	acm "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	btypes "github.com/tendermint/tendermint/cmd/barak/types"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/rpc/client"
	"net/http"
)

// These are convenience functions for a single developer.
// When multiple are involved, the workflow is different.
// (First the command(s) are signed by all validators,
//  and then it is broadcast).
// TODO: Implement a reasonable workflow with multiple validators.

func StartProcess(privKey acm.PrivKey, remote string, command btypes.CommandStartProcess) (response btypes.ResponseStartProcess, err error) {
	nonce, err := GetNonce(remote)
	if err != nil {
		return response, err
	}
	commandBytes, signature := SignCommand(privKey, nonce+1, command)
	_, err = RunAuthCommand(remote, commandBytes, []acm.Signature{signature}, &response)
	return response, err
}

func StopProcess(privKey acm.PrivKey, remote string, command btypes.CommandStopProcess) (response btypes.ResponseStopProcess, err error) {
	nonce, err := GetNonce(remote)
	if err != nil {
		return response, err
	}
	commandBytes, signature := SignCommand(privKey, nonce+1, command)
	_, err = RunAuthCommand(remote, commandBytes, []acm.Signature{signature}, &response)
	return response, err
}

func ListProcesses(privKey acm.PrivKey, remote string, command btypes.CommandListProcesses) (response btypes.ResponseListProcesses, err error) {
	nonce, err := GetNonce(remote)
	if err != nil {
		return response, err
	}
	commandBytes, signature := SignCommand(privKey, nonce+1, command)
	_, err = RunAuthCommand(remote, commandBytes, []acm.Signature{signature}, &response)
	return response, err
}

func OpenListener(privKey acm.PrivKey, remote string, command btypes.CommandOpenListener) (response btypes.ResponseOpenListener, err error) {
	nonce, err := GetNonce(remote)
	if err != nil {
		return response, err
	}
	commandBytes, signature := SignCommand(privKey, nonce+1, command)
	_, err = RunAuthCommand(remote, commandBytes, []acm.Signature{signature}, &response)
	return response, err
}

func CloseListener(privKey acm.PrivKey, remote string, command btypes.CommandCloseListener) (response btypes.ResponseCloseListener, err error) {
	nonce, err := GetNonce(remote)
	if err != nil {
		return response, err
	}
	commandBytes, signature := SignCommand(privKey, nonce+1, command)
	_, err = RunAuthCommand(remote, commandBytes, []acm.Signature{signature}, &response)
	return response, err
}

func DownloadFile(privKey acm.PrivKey, remote string, command btypes.CommandServeFile, outPath string) (n int64, err error) {
	// Create authCommandJSONBytes
	nonce, err := GetNonce(remote)
	if err != nil {
		return 0, err
	}
	commandBytes, signature := SignCommand(privKey, nonce+1, command)
	authCommand := btypes.AuthCommand{
		CommandJSONStr: string(commandBytes),
		Signatures:     []acm.Signature{signature},
	}
	authCommandJSONBytes := binary.JSONBytes(authCommand)
	// Make request and write to outPath.
	httpResponse, err := http.PostForm(remote+"/download", url.Values{"auth_command": {string(authCommandJSONBytes)}})
	if err != nil {
		return 0, err
	}
	defer httpResponse.Body.Close()
	outFile, err := os.OpenFile(outPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return 0, err
	}
	defer outFile.Close()
	n, err = io.Copy(outFile, httpResponse.Body)
	if err != nil {
		return 0, err
	}
	return n, nil
}

//-----------------------------------------------------------------------------

// Utility method to get nonce from the remote.
// The next command should include the returned nonce+1 as nonce.
func GetNonce(remote string) (uint64, error) {
	response, err := GetStatus(remote)
	return response.Nonce, err
}

func GetStatus(remote string) (response btypes.ResponseStatus, err error) {
	_, err = rpcclient.Call(remote, "status", Arr(), &response)
	if err != nil {
		return response, fmt.Errorf("Error fetching nonce from remote %v:\n  %v", remote, err)
	}
	return response, nil
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
	return rpcclient.Call(remote, "run", Arr(authCommand), dest)
}
