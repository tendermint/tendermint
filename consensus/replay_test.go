package consensus

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/tendermint/tendermint/types"
)

/*
	The easiest way to generate this data is to rm ~/.tendermint,
	copy ~/.tendermint_test/somedir/* to ~/.tendermint,
	run `tendermint unsafe_reset_all`,
	set ~/.tendermint/config.toml to use "leveldb" (to create a cswal in data/),
	run `make install`,
	and to run a local node.

	If you need to change the signatures, you can use a script as follows:
	The privBytes comes from config/tendermint_test/...

	```
	package main

	import (
		"encoding/hex"
		"fmt"

		"github.com/tendermint/go-crypto"
	)

	func main() {
		signBytes, err := hex.DecodeString("7B22636861696E5F6964223A2274656E6465726D696E745F74657374222C22766F7465223A7B22626C6F636B5F68617368223A2242453544373939433846353044354645383533364334333932464443384537423342313830373638222C22626C6F636B5F70617274735F686561646572223A506172745365747B543A31204236323237323535464632307D2C22686569676874223A312C22726F756E64223A302C2274797065223A327D7D")
		if err != nil {
			panic(err)
		}
		privBytes, err := hex.DecodeString("27F82582AEFAE7AB151CFB01C48BB6C1A0DA78F9BDDA979A9F70A84D074EB07D3B3069C422E19688B45CBFAE7BB009FC0FA1B1EA86593519318B7214853803C8")
		if err != nil {
			panic(err)
		}
		privKey := crypto.PrivKeyEd25519{}
		copy(privKey[:], privBytes)
		signature := privKey.Sign(signBytes)
		signatureEd25519 := signature.(crypto.SignatureEd25519)
		fmt.Printf("Signature Bytes: %X\n", signatureEd25519[:])
	}
	```
*/

var testLog = `
{"time":"2016-08-20T22:06:16.075Z","msg":[3,{"duration":0,"height":1,"round":0,"step":1}]}
{"time":"2016-08-20T22:06:16.077Z","msg":[1,{"height":1,"round":0,"step":"RoundStepPropose"}]}
{"time":"2016-08-20T22:06:16.077Z","msg":[2,{"msg":[17,{"Proposal":{"height":1,"round":0,"block_parts_header":{"total":1,"hash":"BC874710A7514C59E4F460A9621C538CF82C160A"},"pol_round":-1,"pol_block_id":{"hash":"","parts":{"total":0,"hash":""}},"signature":"E5ABDE10A4D3819184850AF85207DFD2DB8A9D3C540C4313C22DFA470AE99CDF43E5511E4FB7AAC040134E27AA8FC42869C655B4D812175ACAC32BAB7E51C906"}}],"peer_key":""}]}
{"time":"2016-08-20T22:06:16.078Z","msg":[2,{"msg":[19,{"Height":1,"Round":0,"Part":{"index":0,"bytes":"0101010F74656E6465726D696E745F746573740101146CA357E0F5D8C00000000000000114C4B01D3810579550997AC5641E759E20D99B51C10001000100","proof":{"aunts":[]}}}],"peer_key":""}]}
{"time":"2016-08-20T22:06:16.079Z","msg":[1,{"height":1,"round":0,"step":"RoundStepPrevote"}]}
{"time":"2016-08-20T22:06:16.079Z","msg":[2,{"msg":[20,{"Vote":{"validator_address":"D028C9981F7A87F3093672BF0D5B0E2A1B3ED456","validator_index":0,"height":1,"round":0,"type":1,"block_id":{"hash":"D98DD4077A50690BAF670E26FD8E56AFC6ACFF4D","parts":{"total":1,"hash":"BC874710A7514C59E4F460A9621C538CF82C160A"}},"signature":"834DE6E3F530178DC695DAE92B2EEF2E31704E58256AF21A27045918A7F225CF59D24B345518C21F67E77559E0953E14DA7372863365242A22BDE2AE1BEABD04"}}],"peer_key":""}]}
{"time":"2016-08-20T22:06:16.080Z","msg":[1,{"height":1,"round":0,"step":"RoundStepPrecommit"}]}
{"time":"2016-08-20T22:06:16.080Z","msg":[2,{"msg":[20,{"Vote":{"validator_address":"D028C9981F7A87F3093672BF0D5B0E2A1B3ED456","validator_index":0,"height":1,"round":0,"type":2,"block_id":{"hash":"D98DD4077A50690BAF670E26FD8E56AFC6ACFF4D","parts":{"total":1,"hash":"BC874710A7514C59E4F460A9621C538CF82C160A"}},"signature":"3028C891510029A00859E4C56E4D0836C9B3E1F5A8F7CFF9E1B36D40E7727A7A64615ACACF1791C453C87E9FBFE66D978566DA92A12C6ABD7307FF5D1430A408"}}],"peer_key":""}]}
`

func TestReplayCatchup(t *testing.T) {
	// write the needed wal to file
	f, err := ioutil.TempFile(os.TempDir(), "replay_test_")
	if err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	_, err = f.WriteString(testLog)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	cs := fixedConsensusState()

	// we've already precommitted on the first block
	// without replay catchup we would be halted here forever
	cs.privValidator.LastHeight = 1 // first block
	cs.privValidator.LastStep = 3   // precommit

	newBlockCh := subscribeToEvent(cs.evsw, "tester", types.EventStringNewBlock(), 0)

	// start timeout and receive routines
	cs.startRoutines(0)

	// open wal and run catchup messages
	openWAL(t, cs, name)
	if err := cs.catchupReplay(cs.Height); err != nil {
		t.Fatalf("Error on catchup replay %v", err)
	}

	after := time.After(time.Second * 15)
	select {
	case <-newBlockCh:
	case <-after:
		t.Fatal("Timed out waiting for new block")
	}
}

func openWAL(t *testing.T, cs *ConsensusState, file string) {
	// open the wal
	wal, err := NewWAL(file, config.GetBool("cswal_light"))
	if err != nil {
		t.Fatal(err)
	}
	wal.exists = true
	cs.wal = wal
}
