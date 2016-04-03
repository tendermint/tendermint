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

var testLog = `{"time":"2016-04-03T11:23:54.387Z","msg":[3,{"duration":972835254,"height":1,"round":0,"step":1}]}
{"time":"2016-04-03T11:23:54.388Z","msg":[1,{"height":1,"round":0,"step":"RoundStepPropose"}]}
{"time":"2016-04-03T11:23:54.388Z","msg":[2,{"msg":[17,{"Proposal":{"height":1,"round":0,"block_parts_header":{"total":1,"hash":"3BA1E90CB868DA6B4FD7F3589826EC461E9EB4EF"},"pol_round":-1,"signature":"3A2ECD5023B21EC144EC16CFF1B992A4321317B83EEDD8969FDFEA6EB7BF4389F38DDA3E7BB109D63A07491C16277A197B241CF1F05F5E485C59882ECACD9E07"}}],"peer_key":""}]}
{"time":"2016-04-03T11:23:54.389Z","msg":[2,{"msg":[19,{"Height":1,"Round":0,"Part":{"index":0,"bytes":"0101010F74656E6465726D696E745F7465737401011441D59F4B718AC00000000000000114C4B01D3810579550997AC5641E759E20D99B51C10001000100","proof":{"aunts":[]}}}],"peer_key":""}]}
{"time":"2016-04-03T11:23:54.390Z","msg":[1,{"height":1,"round":0,"step":"RoundStepPrevote"}]}
{"time":"2016-04-03T11:23:54.390Z","msg":[2,{"msg":[20,{"ValidatorIndex":0,"Vote":{"height":1,"round":0,"type":1,"block_hash":"4291966B8A9DFBA00AEC7C700F2718E61DF4331D","block_parts_header":{"total":1,"hash":"3BA1E90CB868DA6B4FD7F3589826EC461E9EB4EF"},"signature":"47D2A75A4E2F15DB1F0D1B656AC0637AF9AADDFEB6A156874F6553C73895E5D5DC948DBAEF15E61276C5342D0E638DFCB77C971CD282096EA8735A564A90F008"}}],"peer_key":""}]}
{"time":"2016-04-03T11:23:54.392Z","msg":[1,{"height":1,"round":0,"step":"RoundStepPrecommit"}]}
{"time":"2016-04-03T11:23:54.392Z","msg":[2,{"msg":[20,{"ValidatorIndex":0,"Vote":{"height":1,"round":0,"type":2,"block_hash":"4291966B8A9DFBA00AEC7C700F2718E61DF4331D","block_parts_header":{"total":1,"hash":"3BA1E90CB868DA6B4FD7F3589826EC461E9EB4EF"},"signature":"39147DA595F08B73CF8C899967C8403B5872FD9042FFA4E239159E0B6C5D9665C9CA81D766EACA2AE658872F94C2FCD1E34BF51859CD5B274DA8512BACE4B50D"}}],"peer_key":""}]}
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

	after := time.After(time.Second * 2)
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
