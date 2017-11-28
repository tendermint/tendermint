package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/tendermint/abci/types"
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

func ensureABCIIsUp(subCommand string, n int) error {
	var err error
	for i := 0; i < n; i++ {
		cmd := exec.Command("bash", "-c", "abci-cli", subCommand)
		_, err = cmd.CombinedOutput()
		if err == nil {
			break
		}
		<-time.After(500 * time.Second)
	}
	return err
}

func testCounter() {
	abciApp := os.Getenv("ABCI_APP")
	if abciApp == "" {
		panic("No ABCI_APP specified")
	}

	fmt.Printf("Running %s test with abci=%s\n", abciApp, abciType)
	cmd := exec.Command("bash", "-c", fmt.Sprintf("abci-cli %s", abciApp))
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		log.Fatalf("starting %q err: %v", abciApp, err)
	}
	defer cmd.Wait()
	defer cmd.Process.Kill()

	if err := ensureABCIIsUp("echo", 5); err != nil {
		log.Fatalf("echo failed: %v", err)
	}

	client := startClient(abciType)
	defer client.Stop()

	setOption(client, "serial", "on")
	commit(client, nil)
	deliverTx(client, []byte("abc"), types.CodeType_BadNonce, nil)
	commit(client, nil)
	deliverTx(client, []byte{0x00}, types.CodeType_OK, nil)
	commit(client, []byte{0, 0, 0, 0, 0, 0, 0, 1})
	deliverTx(client, []byte{0x00}, types.CodeType_BadNonce, nil)
	deliverTx(client, []byte{0x01}, types.CodeType_OK, nil)
	deliverTx(client, []byte{0x00, 0x02}, types.CodeType_OK, nil)
	deliverTx(client, []byte{0x00, 0x03}, types.CodeType_OK, nil)
	deliverTx(client, []byte{0x00, 0x00, 0x04}, types.CodeType_OK, nil)
	deliverTx(client, []byte{0x00, 0x00, 0x06}, types.CodeType_BadNonce, nil)
	commit(client, []byte{0, 0, 0, 0, 0, 0, 0, 5})
}
