package consensus

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/tendermint/tendermint/types"
)

/*
	The easiest way to generate this data is to copy ~/.tendermint_test/somedir/* to ~/.tendermint
	and to run a local node.
	Be sure to set the db to "leveldb" to create a cswal file in ~/.tendermint/data/cswal.

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

var testLog = `{"time":"2016-07-01T21:44:23.626Z","msg":[3,{"duration":0,"height":1,"round":0,"step":1}]}
{"time":"2016-07-01T21:44:23.631Z","msg":[1,{"height":1,"round":0,"step":"RoundStepPropose"}]}
{"time":"2016-07-01T21:44:23.631Z","msg":[2,{"msg":[17,{"Proposal":{"height":1,"round":0,"block_parts_header":{"total":1,"hash":"108807E7DC79BC7716CCC93217AC2B81BB8C9508"},"pol_round":-1,"signature":"1B22E998F5F4C426CB29F8AADFB012E364780B3FFDABA3B68498903EDBACECF152F45E3E3CD53A9476577E7043122E506494F4581ACDC73FE9643A791F977A07"}}],"peer_key":""}]}
{"time":"2016-07-01T21:44:23.632Z","msg":[2,{"msg":[19,{"Height":1,"Round":0,"Part":{"index":0,"bytes":"0101010F74656E6465726D696E745F746573740101145D4921EB88A8C00000000000000114C4B01D3810579550997AC5641E759E20D99B51C10001000100","proof":{"aunts":[]}}}],"peer_key":""}]}
{"time":"2016-07-01T21:44:23.633Z","msg":[1,{"height":1,"round":0,"step":"RoundStepPrevote"}]}
{"time":"2016-07-01T21:44:23.633Z","msg":[2,{"msg":[20,{"Vote":{"validator_address":"D028C9981F7A87F3093672BF0D5B0E2A1B3ED456","validator_index":0,"height":1,"round":0,"type":1,"block_hash":"C421180ECD00F7FEFF8D720AE8CE891B02E85EA3","block_parts_header":{"total":1,"hash":"108807E7DC79BC7716CCC93217AC2B81BB8C9508"},"signature":"816DEAB87DB1BCA284EAEC7968B9EAE057025478D3176E174E0696C74CB2DD58EA402A3E144CA2292CCF05741097395E42EB8DA492EC73CAE8AB7C4486F99609"}}],"peer_key":""}]}
{"time":"2016-07-01T21:44:23.636Z","msg":[1,{"height":1,"round":0,"step":"RoundStepPrecommit"}]}
{"time":"2016-07-01T21:44:23.636Z","msg":[2,{"msg":[20,{"Vote":{"validator_address":"D028C9981F7A87F3093672BF0D5B0E2A1B3ED456","validator_index":0,"height":1,"round":0,"type":2,"block_hash":"C421180ECD00F7FEFF8D720AE8CE891B02E85EA3","block_parts_header":{"total":1,"hash":"108807E7DC79BC7716CCC93217AC2B81BB8C9508"},"signature":"BCF879FD579D03282ACF8B5C633DE9548BD93457AC7D0E69A3B13C4BF0E5581BBA20D29DF89FA20AF01F3DD39D65934AABB3B31B5E749B9EA3E4C11935E7610F"}}],"peer_key":""}]}
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
