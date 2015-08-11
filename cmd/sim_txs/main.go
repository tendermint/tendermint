package main

import (
	"encoding/hex"
	"flag"
	"fmt"

	acm "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	cclient "github.com/tendermint/tendermint/rpc/core_client"
	"github.com/tendermint/tendermint/types"
)

const Version = "0.0.1"
const sleepSeconds = 1 // Every second

// Parse command-line options
func parseFlags() (privKeyHex string, numAccounts int, remote string) {
	var version bool
	flag.StringVar(&privKeyHex, "priv-key", "", "Private key bytes in HEX")
	flag.IntVar(&numAccounts, "num-accounts", 1000, "Deterministically generates this many sub-accounts")
	flag.StringVar(&remote, "remote", "localhost:46657", "Remote RPC host:port")
	flag.BoolVar(&version, "version", false, "Version")
	flag.Parse()
	if version {
		Exit(Fmt("sim_txs version %v", Version))
	}
	return
}

func main() {

	// Read options
	privKeyHex, numAccounts, remote := parseFlags()

	// Print args.
	// fmt.Println(privKeyHex, numAccounts, remote)

	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		panic(err)
	}
	var privKeyArray [64]byte
	copy(privKeyArray[:], privKeyBytes)
	root := acm.GenPrivAccountFromPrivKeyBytes(&privKeyArray)
	fmt.Println("Computed address: %X", root.Address)

	// Get root account.
	rootAccount, err := getAccount(remote, root.Address)
	if err != nil {
		fmt.Println(Fmt("Root account %X does not exist: %v", root.Address, err))
		return
	} else {
		fmt.Println("Root account", rootAccount)
	}

	// Load all accounts
	accounts := make([]*acm.Account, numAccounts+1)
	accounts[0] = rootAccount
	privAccounts := make([]*acm.PrivAccount, numAccounts+1)
	privAccounts[0] = root
	for i := 1; i < numAccounts+1; i++ {
		privAccounts[i] = root.Generate(i)
		account, err := getAccount(remote, privAccounts[i].Address)
		if err != nil {
			fmt.Println("Error", err)
			return
		} else {
			accounts[i] = account
		}
	}

	// Test: send from root to accounts[1]
	sendTx := makeRandomTransaction(10, rootAccount.Sequence+1, root, 2, accounts)
	fmt.Println(sendTx)

	wsClient := cclient.NewWSClient("ws://" + remote + "/websocket")
	_, err = wsClient.Start()
	if err != nil {
		Exit(Fmt("Failed to establish websocket connection: %v", err))
	}
	wsClient.Subscribe(types.EventStringAccInput(sendTx.Inputs[0].Address))

	go func() {
		for {
			foo := <-wsClient.EventsCh
			fmt.Println("!!", foo)
		}
	}()

	err = broadcastSendTx(remote, sendTx)
	if err != nil {
		Exit(Fmt("Failed to broadcast SendTx: %v", err))
		return
	}

	// Trap signal
	TrapSignal(func() {
		fmt.Println("sim_txs shutting down")
	})
}

func getAccount(remote string, address []byte) (*acm.Account, error) {
	// var account *acm.Account = new(acm.Account)
	account, err := rpcclient.Call("http://"+remote, "get_account", []interface{}{address}, (*acm.Account)(nil))
	if err != nil {
		return nil, err
	}
	if account.(*acm.Account) == nil {
		return &acm.Account{Address: address}, nil
	} else {
		return account.(*acm.Account), nil
	}
}

func broadcastSendTx(remote string, sendTx *types.SendTx) error {
	receipt, err := rpcclient.Call("http://"+remote, "broadcast_tx", []interface{}{sendTx}, (*ctypes.Receipt)(nil))
	if err != nil {
		return err
	}
	fmt.Println("Broadcast receipt:", receipt)
	return nil
}

// Make a random send transaction from srcIndex to N other accounts.
// balance: balance to send from input
// sequence: sequence to sign with
// inputPriv: input privAccount
func makeRandomTransaction(balance int64, sequence int, inputPriv *acm.PrivAccount, sendCount int, accounts []*acm.Account) *types.SendTx {

	if sendCount >= len(accounts) {
		PanicSanity("Cannot make tx with sendCount >= len(accounts)")
	}

	// Remember which accounts were chosen
	accMap := map[string]struct{}{}
	accMap[string(inputPriv.Address)] = struct{}{}

	// Find a selection of accounts to send to
	outputs := []*acm.Account{}
	for i := 0; i < sendCount; i++ {
		for {
			idx := RandInt() % len(accounts)
			account := accounts[idx]
			if _, ok := accMap[string(account.Address)]; ok {
				continue
			}
			accMap[string(account.Address)] = struct{}{}
			outputs = append(outputs, account)
			break
		}
	}

	// Construct SendTx
	sendTx := types.NewSendTx()
	err := sendTx.AddInputWithNonce(inputPriv.PubKey, balance, sequence)
	if err != nil {
		panic(err)
	}
	for _, output := range outputs {
		sendTx.AddOutput(output.Address, balance/int64(len(outputs)))
	}

	// Sign SendTx
	sendTx.SignInput("tendermint_testnet_9", 0, inputPriv)

	return sendTx
}
