package consensus

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/tendermint/tendermint/types"
)

/*
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

var testLog = `{"time":"2016-01-18T20:46:00.774Z","msg":[3,{"duration":982632969,"height":1,"round":0,"step":1}]}
{"time":"2016-01-18T20:46:00.776Z","msg":[1,{"height":1,"round":0,"step":"RoundStepPropose"}]}
{"time":"2016-01-18T20:46:00.776Z","msg":[2,{"msg":[17,{"Proposal":{"height":1,"round":0,"block_parts_header":{"total":1,"hash":"B6227255FF20758326B0B7DFF529F20E33E58F45"},"pol_round":-1,"signature":"A1803A1364F6398C154FE45D5649A89129039F18A0FE42B211BADFDF6E81EA53F48F83D3610DDD848C3A5284D3F09BDEB26FA1D856BDF70D48C507BF2453A70E"}}],"peer_key":""}]}
{"time":"2016-01-18T20:46:00.777Z","msg":[2,{"msg":[19,{"Height":1,"Round":0,"Part":{"index":0,"bytes":"0101010F74656E6465726D696E745F746573740101142AA030B15DDFC000000000000000000000000000000114C4B01D3810579550997AC5641E759E20D99B51C10001000100","proof":{"aunts":[]}}}],"peer_key":""}]}
{"time":"2016-01-18T20:46:00.781Z","msg":[1,{"height":1,"round":0,"step":"RoundStepPrevote"}]}
{"time":"2016-01-18T20:46:00.781Z","msg":[2,{"msg":[20,{"ValidatorIndex":0,"Vote":{"height":1,"round":0,"type":1,"block_hash":"BE5D799C8F50D5FE8536C4392FDC8E7B3B180768","block_parts_header":{"total":1,"hash":"B6227255FF20758326B0B7DFF529F20E33E58F45"},"signature":"A63A15D9155C6C97762E65559E8CD34D853ED5FE22DA8540519453F9B7BE23C9287110929EDAA430DA30183768C60D4CF2811128C9D2E4B6A173EFE28FB54F05"}}],"peer_key":""}]}
{"time":"2016-01-18T20:46:00.786Z","msg":[1,{"height":1,"round":0,"step":"RoundStepPrecommit"}]}
{"time":"2016-01-18T20:46:00.786Z","msg":[2,{"msg":[20,{"ValidatorIndex":0,"Vote":{"height":1,"round":0,"type":2,"block_hash":"BE5D799C8F50D5FE8536C4392FDC8E7B3B180768","block_parts_header":{"total":1,"hash":"B6227255FF20758326B0B7DFF529F20E33E58F45"},"signature":"7190035C558F7173946581E6121E1EEB6EE91F03F04F2858FD5CBDAC006521FDC50366ED920B69C7357D9EF7621F9578F00E5350757CBC0AE6D441C39170CC07"}}],"peer_key":""}]}
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
