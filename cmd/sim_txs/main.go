package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"time"

	acm "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

const Version = "0.0.1"
const sleepSeconds = 1 // Every second

// Parse command-line options
func parseFlags() (privKeyHex string, numAccounts int, remote string, version bool) {
	flag.StringVar(&privKeyHex, "priv-key", "", "Private key bytes in HEX")
	flag.IntVar(&numAccounts, "num-accounts", 1000, "Deterministically generates this many sub-accounts")
	flag.StringVar(&remote, "remote", "http://localhost:46657", "Remote RPC host:port")
	flag.BoolVar(&version, "version", false, "Version")
	flag.Parse()
	return
}

func main() {

	// Read options
	privKeyHex, numAccounts, remote, version := parseFlags()
	if version {
		fmt.Println(Fmt("sim_txs version %v", Version))
		return
	}

	// Print args.
	// fmt.Println(privKeyHex, numAccounts, remote, version)

	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		panic(err)
	}
	root := acm.GenPrivAccountFromPrivKeyBytes(privKeyBytes)
	fmt.Println("Computed address: %X", root.Address)

	// Get root account.
	rootAccount, err := getAccount(remote, root.Address)
	if err != nil {
		fmt.Println(Fmt("Root account does not exist: %X", root.Address))
		return
	} else {
		fmt.Println("Root account", rootAccount)
	}

	go func() {
		// Construct a new send Tx
		accounts := make([]*acm.Account, numAccounts)
		privAccounts := make([]*acm.PrivAccount, numAccounts)
		for i := 0; i < numAccounts; i++ {
			privAccounts[i] = root.Generate(i)
			account, err := getAccount(remote, privAccounts[i].Address)
			if err != nil {
				fmt.Println("Error", err)
				return
			} else {
				accounts[i] = account
			}
		}
		for {
			sendTx := makeRandomTransaction(rootAccount, root, accounts, privAccounts)
			// Broadcast it.
			err := broadcastSendTx(remote, sendTx)
			if err != nil {
				Exit(Fmt("Failed to broadcast SendTx: %v", err))
				return
			}
			// Broadcast 1 tx!
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Trap signal
	TrapSignal(func() {
		fmt.Println("sim_txs shutting down")
	})
}

func getAccount(remote string, address []byte) (*acm.Account, error) {
	// var account *acm.Account = new(acm.Account)
	account, err := rpcclient.Call(remote, "get_account", []interface{}{address}, (*acm.Account)(nil))
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
	receipt, err := rpcclient.Call(remote, "broadcast_tx", []interface{}{sendTx}, (*ctypes.Receipt)(nil))
	if err != nil {
		return err
	}
	fmt.Println("Broadcast receipt:", receipt)
	return nil
}

func makeRandomTransaction(rootAccount *acm.Account, rootPrivAccount *acm.PrivAccount, accounts []*acm.Account, privAccounts []*acm.PrivAccount) *types.SendTx {
	allAccounts := append(accounts, rootAccount)
	allPrivAccounts := append(privAccounts, rootPrivAccount)

	// Find accout with the most money
	inputBalance := int64(0)
	inputAccount := (*acm.Account)(nil)
	inputPrivAccount := (*acm.PrivAccount)(nil)
	for i, account := range allAccounts {
		if account == nil {
			continue
		}
		if inputBalance < account.Balance {
			inputBalance = account.Balance
			inputAccount = account
			inputPrivAccount = allPrivAccounts[i]
		}
	}
	if inputAccount == nil {
		Exit("No accounts have any money")
		return nil
	}

	// Find a selection of accounts to send to
	outputAccounts := map[string]*acm.Account{}
	for i := 0; i < 2; i++ {
		for {
			idx := RandInt() % len(accounts)
			if bytes.Equal(accounts[idx].Address, inputAccount.Address) {
				continue
			}
			if _, ok := outputAccounts[string(accounts[idx].Address)]; ok {
				continue
			}
			outputAccounts[string(accounts[idx].Address)] = accounts[idx]
			break
		}
	}

	// Construct SendTx
	sendTx := types.NewSendTx()
	err := sendTx.AddInputWithNonce(inputPrivAccount.PubKey, inputAccount.Balance, inputAccount.Sequence+1)
	if err != nil {
		panic(err)
	}
	for _, outputAccount := range outputAccounts {
		sendTx.AddOutput(outputAccount.Address, inputAccount.Balance/int64(len(outputAccounts)))
		// XXX FIXME???
		outputAccount.Balance += inputAccount.Balance / int64(len(outputAccounts))
	}

	// Sign SendTx
	sendTx.SignInput("tendermint_testnet_7", 0, inputPrivAccount)

	// Hack: Listen for events or create a new RPC call for this.
	// XXX FIXME
	inputAccount.Sequence += 1
	inputAccount.Balance = 0 // FIXME???

	return sendTx
}
