package consensus

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
)

var data_dir = "test_data" // TODO

type testCase struct {
	name    string
	log     string       //full cswal
	stepMap map[int]int8 // map lines of log to privval step
}

// mapping for one validator and blocks with one part
var baseStepMap = map[int]int8{
	0: 0,
	1: 0,
	2: 1,
	3: 1,
	4: 1,
	5: 2,
	6: 2,
	7: 3,
}

var testCases = []*testCase{
	&testCase{
		name:    "empty_block",
		stepMap: baseStepMap,
	},
	&testCase{
		name:    "small_block",
		stepMap: baseStepMap,
	},
}

// populate test logs by reading files
func init() {
	for _, c := range testCases {
		c.log = readWAL(path.Join(data_dir, c.name+".cswal"))
	}
}

func readWAL(p string) string {
	b, err := ioutil.ReadFile(p)
	if err != nil {
		panic(err)
	}
	return string(b)
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

func waitForBlock(newBlockCh chan interface{}, thisCase *testCase, i int) {
	after := time.After(time.Second * 10)
	select {
	case <-newBlockCh:
	case <-after:
		panic(Fmt("Timed out waiting for new block for case '%s' line %d", thisCase.name, i))
	}
}

func runReplayTest(t *testing.T, cs *ConsensusState, fileName string, newBlockCh chan interface{},
	thisCase *testCase, i int) {

	cs.config.Set("cswal", fileName)
	cs.Start()
	// Wait to make a new block.
	// This is just a signal that we haven't halted; its not something contained in the WAL itself.
	// Assuming the consensus state is running, replay of any WAL, including the empty one,
	// should eventually be followed by a new block, or else something is wrong
	waitForBlock(newBlockCh, thisCase, i)
	cs.Stop()
}

func setupReplayTest(thisCase *testCase, nLines int, crashAfter bool) (*ConsensusState, chan interface{}, string, string) {
	fmt.Println("-------------------------------------")
	log.Notice(Fmt("Starting replay test of %d lines of WAL (crash before write)", nLines))

	lineStep := nLines
	if crashAfter {
		lineStep -= 1
	}

	split := strings.Split(thisCase.log, "\n")
	lastMsg := split[nLines]

	// we write those lines up to (not including) one with the signature
	fileName := writeWAL(strings.Join(split[:nLines], "\n") + "\n")

	cs := fixedConsensusState()

	// set the last step according to when we crashed vs the wal
	cs.privValidator.LastHeight = 1 // first block
	cs.privValidator.LastStep = thisCase.stepMap[lineStep]

	fmt.Println("LAST STEP", cs.privValidator.LastStep)

	newBlockCh := subscribeToEvent(cs.evsw, "tester", types.EventStringNewBlock(), 1)

	return cs, newBlockCh, lastMsg, fileName
}

//-----------------------------------------------
// Test the log at every iteration, and set the privVal last step
// as if the log was written after signing, before the crash

func TestReplayCrashAfterWrite(t *testing.T) {
	for _, thisCase := range testCases {
		split := strings.Split(thisCase.log, "\n")
		for i := 0; i < len(split)-1; i++ {
			cs, newBlockCh, _, f := setupReplayTest(thisCase, i+1, true)
			runReplayTest(t, cs, f, newBlockCh, thisCase, i+1)
		}
	}
}

//-----------------------------------------------
// Test the log as if we crashed after signing but before writing.
// This relies on privValidator.LastSignature being set

func TestReplayCrashBeforeWritePropose(t *testing.T) {
	for _, thisCase := range testCases {
		lineNum := 2
		cs, newBlockCh, proposalMsg, f := setupReplayTest(thisCase, lineNum, false) // propose
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
		runReplayTest(t, cs, f, newBlockCh, thisCase, lineNum)
	}
}

func TestReplayCrashBeforeWritePrevote(t *testing.T) {
	for _, thisCase := range testCases {
		lineNum := 5
		cs, newBlockCh, voteMsg, f := setupReplayTest(thisCase, lineNum, false) // prevote
		types.AddListenerForEvent(cs.evsw, "tester", types.EventStringCompleteProposal(), func(data types.TMEventData) {
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
		runReplayTest(t, cs, f, newBlockCh, thisCase, lineNum)
	}
}

func TestReplayCrashBeforeWritePrecommit(t *testing.T) {
	for _, thisCase := range testCases {
		lineNum := 7
		cs, newBlockCh, voteMsg, f := setupReplayTest(thisCase, lineNum, false) // precommit
		types.AddListenerForEvent(cs.evsw, "tester", types.EventStringPolka(), func(data types.TMEventData) {
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
		runReplayTest(t, cs, f, newBlockCh, thisCase, lineNum)
	}
}
