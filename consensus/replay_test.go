package consensus

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-events"
	"github.com/tendermint/go-wire"
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

// map lines in the above wal to privVal step
var mapPrivValStep = map[int]int8{
	0: 0,
	1: 0,
	2: 1,
	3: 1,
	4: 1,
	5: 2,
	6: 2,
	7: 3,
}

func writeWAL(log string) string {
	fmt.Println("writing", log)
	// write the needed wal to file
	f, err := ioutil.TempFile(os.TempDir(), "replay_test_")
	if err != nil {
		panic(err)
	}

	_, err = f.WriteString(log)
	if err != nil {
		panic(err)
	}
	name := f.Name()
	f.Close()
	return name
}

func waitForBlock(newBlockCh chan interface{}) {
	after := time.After(time.Second * 10)
	select {
	case <-newBlockCh:
	case <-after:
		panic("Timed out waiting for new block")
	}
}

func runReplayTest(t *testing.T, cs *ConsensusState, fileName string, newBlockCh chan interface{}) {
	cs.config.Set("cswal", fileName)
	cs.Start()
	// Wait to make a new block.
	// This is just a signal that we haven't halted; its not something contained in the WAL itself.
	// Assuming the consensus state is running, replay of any WAL, including the empty one,
	// should eventually be followed by a new block, or else something is wrong
	waitForBlock(newBlockCh)
	cs.Stop()
}

func setupReplayTest(nLines int, crashAfter bool) (*ConsensusState, chan interface{}, string, string) {
	fmt.Println("-------------------------------------")
	log.Notice(Fmt("Starting replay test of %d lines of WAL (crash before write)", nLines))

	lineStep := nLines
	if crashAfter {
		lineStep -= 1
	}

	split := strings.Split(testLog, "\n")
	lastMsg := split[nLines]

	// we write those lines up to (not including) one with the signature
	fileName := writeWAL(strings.Join(split[:nLines], "\n") + "\n")

	cs := fixedConsensusState()

	// set the last step according to when we crashed vs the wal
	cs.privValidator.LastHeight = 1 // first block
	cs.privValidator.LastStep = mapPrivValStep[lineStep]

	fmt.Println("LAST STEP", cs.privValidator.LastStep)

	newBlockCh := subscribeToEvent(cs.evsw, "tester", types.EventStringNewBlock(), 1)

	return cs, newBlockCh, lastMsg, fileName
}

//-----------------------------------------------
// Test the log at every iteration, and set the privVal last step
// as if the log was written after signing, before the crash

func TestReplayCrashAfterWrite(t *testing.T) {
	split := strings.Split(testLog, "\n")
	for i := 0; i < len(split)-1; i++ {
		cs, newBlockCh, _, f := setupReplayTest(i+1, true)
		runReplayTest(t, cs, f, newBlockCh)
	}
}

//-----------------------------------------------
// Test the log as if we crashed after signing but before writing.
// This relies on privValidator.LastSignature being set

func TestReplayCrashBeforeWritePropose(t *testing.T) {
	cs, newBlockCh, proposalMsg, f := setupReplayTest(2, false) // propose
	// Set LastSig
	var err error
	var msg ConsensusLogMessage
	wire.ReadJSON(&msg, []byte(proposalMsg), &err)
	proposal := msg.Msg.(msgInfo).Msg.(*ProposalMessage)
	if err != nil {
		t.Fatalf("Error reading json data: %v", err)
	}
	cs.privValidator.LastSignBytes = types.SignBytes(cs.state.ChainID, proposal.Proposal)
	cs.privValidator.LastSignature = proposal.Proposal.Signature
	runReplayTest(t, cs, f, newBlockCh)
}

func TestReplayCrashBeforeWritePrevote(t *testing.T) {
	cs, newBlockCh, voteMsg, f := setupReplayTest(5, false) // prevote
	cs.evsw.AddListenerForEvent("tester", types.EventStringCompleteProposal(), func(data events.EventData) {
		// Set LastSig
		var err error
		var msg ConsensusLogMessage
		wire.ReadJSON(&msg, []byte(voteMsg), &err)
		vote := msg.Msg.(msgInfo).Msg.(*VoteMessage)
		if err != nil {
			t.Fatalf("Error reading json data: %v", err)
		}
		cs.privValidator.LastSignBytes = types.SignBytes(cs.state.ChainID, vote.Vote)
		cs.privValidator.LastSignature = vote.Vote.Signature
	})
	runReplayTest(t, cs, f, newBlockCh)
}

func TestReplayCrashBeforeWritePrecommit(t *testing.T) {
	cs, newBlockCh, voteMsg, f := setupReplayTest(7, false) // precommit
	cs.evsw.AddListenerForEvent("tester", types.EventStringPolka(), func(data events.EventData) {
		// Set LastSig
		var err error
		var msg ConsensusLogMessage
		wire.ReadJSON(&msg, []byte(voteMsg), &err)
		vote := msg.Msg.(msgInfo).Msg.(*VoteMessage)
		if err != nil {
			t.Fatalf("Error reading json data: %v", err)
		}
		cs.privValidator.LastSignBytes = types.SignBytes(cs.state.ChainID, vote.Vote)
		cs.privValidator.LastSignature = vote.Vote.Signature
	})
	runReplayTest(t, cs, f, newBlockCh)
}
