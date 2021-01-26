package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/types"
)

var abciType string

func init() {
	abciType = os.Getenv("ABCI")
	if abciType == "" {
		abciType = "socket"
	}
}

func main() {
	testCounter()
}

const (
	maxABCIConnectTries = 10
)

func ensureABCIIsUp(typ string, n int) error {
	var err error
	cmdString := "abci-cli echo hello"
	if typ == "grpc" {
		cmdString = "abci-cli --abci grpc echo hello"
	}

	for i := 0; i < n; i++ {
		cmd := exec.Command("bash", "-c", cmdString)
		_, err = cmd.CombinedOutput()
		if err == nil {
			break
		}
		<-time.After(500 * time.Millisecond)
	}
	return err
}

func testCounter() {
	abciApp := os.Getenv("ABCI_APP")
	if abciApp == "" {
		panic("No ABCI_APP specified")
	}

	fmt.Printf("Running %s test with abci=%s\n", abciApp, abciType)
	subCommand := fmt.Sprintf("abci-cli %s", abciApp)
	cmd := exec.Command("bash", "-c", subCommand)
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		log.Fatalf("starting %q err: %v", abciApp, err)
	}
	defer func() {
		if err := cmd.Process.Kill(); err != nil {
			log.Printf("error on process kill: %v", err)
		}
		if err := cmd.Wait(); err != nil {
			log.Printf("error while waiting for cmd to exit: %v", err)
		}
	}()

	if err := ensureABCIIsUp(abciType, maxABCIConnectTries); err != nil {
		log.Fatalf("echo failed: %v", err) //nolint:gocritic
	}

	client := startClient(abciType)
	defer func() {
		if err := client.Stop(); err != nil {
			log.Printf("error trying client stop: %v", err)
		}
	}()

	setOption(client, "serial", "on")
	commit(client, nil)
	deliverTx(client, []byte("abc"), code.CodeTypeBadNonce, nil)
	commit(client, nil)
	deliverTx(client, []byte{0x00}, types.CodeTypeOK, nil)
	commit(client, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})
	deliverTx(client, []byte{0x00}, code.CodeTypeBadNonce, nil)
	deliverTx(client, []byte{0x01}, types.CodeTypeOK, nil)
	deliverTx(client, []byte{0x00, 0x02}, types.CodeTypeOK, nil)
	deliverTx(client, []byte{0x00, 0x03}, types.CodeTypeOK, nil)
	deliverTx(client, []byte{0x00, 0x00, 0x04}, types.CodeTypeOK, nil)
	deliverTx(client, []byte{0x00, 0x00, 0x06}, code.CodeTypeBadNonce, nil)
	commit(client, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5})
}
