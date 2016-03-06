package consensus

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/tendermint/tendermint/types"
)

var testLog = `{"time":"2016-01-18T20:46:00.774Z","msg":[3,{"duration":982632969,"height":1,"round":0,"step":1}]}
{"time":"2016-01-18T20:46:00.776Z","msg":[1,{"height":1,"round":0,"step":"RoundStepPropose"}]}
{"time":"2016-01-18T20:46:00.776Z","msg":[2,{"msg":[17,{"Proposal":{"height":1,"round":0,"block_parts_header":{"total":1,"hash":"B6227255FF20758326B0B7DFF529F20E33E58F45"},"pol_round":-1,"signature":"A1803A1364F6398C154FE45D5649A89129039F18A0FE42B211BADFDF6E81EA53F48F83D3610DDD848C3A5284D3F09BDEB26FA1D856BDF70D48C507BF2453A70E"}}],"peer_key":""}]}
{"time":"2016-01-18T20:46:00.777Z","msg":[2,{"msg":[19,{"Height":1,"Round":0,"Part":{"index":0,"bytes":"0101010F74656E6465726D696E745F746573740101142AA030B15DDFC000000000000000000000000000000114C4B01D3810579550997AC5641E759E20D99B51C10001000100","proof":{"aunts":[]}}}],"peer_key":""}]}
{"time":"2016-01-18T20:46:00.781Z","msg":[1,{"height":1,"round":0,"step":"RoundStepPrevote"}]}
{"time":"2016-01-18T20:46:00.781Z","msg":[2,{"msg":[20,{"ValidatorIndex":0,"Vote":{"height":1,"round":0,"type":1,"block_hash":"E05D1DB8DEC7CDA507A42C8FF208EE4317C663F6","block_parts_header":{"total":1,"hash":"B6227255FF20758326B0B7DFF529F20E33E58F45"},"signature":"88F5708C802BEE54EFBF438967FBC6C6EAAFC41258A85D92B9B055481175BE9FA71007B1AAF2BFBC3BF3CC0542DB48A9812324B7BBA7307446CCDBF029077F07"}}],"peer_key":""}]}
{"time":"2016-01-18T20:46:00.786Z","msg":[1,{"height":1,"round":0,"step":"RoundStepPrecommit"}]}
{"time":"2016-01-18T20:46:00.786Z","msg":[2,{"msg":[20,{"ValidatorIndex":0,"Vote":{"height":1,"round":0,"type":2,"block_hash":"E05D1DB8DEC7CDA507A42C8FF208EE4317C663F6","block_parts_header":{"total":1,"hash":"B6227255FF20758326B0B7DFF529F20E33E58F45"},"signature":"65B0C9D2A8C9919FC9B036F82C3F1818E706E8BC066A78D99D3316E4814AB06594841E387B323AA7773F926D253C1E4D4A0930F7A8C8AE1E838CA15C673B2B02"}}],"peer_key":""}]}
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

	cs.enterNewRound(cs.Height, cs.Round)

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
