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

var data_dir = path.Join(GoPath, "src/github.com/tendermint/tendermint/consensus", "test_data")

// the priv validator changes step at these lines for a block with 1 val and 1 part
var baseStepChanges = []int{2, 5, 7}

// test recovery from each line in each testCase
var testCases = []*testCase{
	newTestCase("empty_block", baseStepChanges),  // empty block (has 1 block part)
	newTestCase("small_block1", baseStepChanges), // small block with txs in 1 block part
	newTestCase("small_block2", []int{2, 7, 9}),  // small block with txs across 3 smaller block parts
}

type testCase struct {
	name    string
	log     string       //full cswal
	stepMap map[int]int8 // map lines of log to privval step

	proposeLine   int
	prevoteLine   int
	precommitLine int
}

func newTestCase(name string, stepChanges []int) *testCase {
	if len(stepChanges) != 3 {
		panic(Fmt("a full wal has 3 step changes! Got array %v", stepChanges))
	}
	return &testCase{
		name:    name,
		log:     readWAL(path.Join(data_dir, name+".cswal")),
		stepMap: newMapFromChanges(stepChanges),

		proposeLine:   stepChanges[0],
		prevoteLine:   stepChanges[1],
		precommitLine: stepChanges[2],
	}
}

func newMapFromChanges(changes []int) map[int]int8 {
	changes = append(changes, changes[2]+1) // so we add the last step change to the map
	m := make(map[int]int8)
	var count int
	for changeNum, nextChange := range changes {
		for ; count < nextChange; count++ {
			m[count] = int8(changeNum)
		}
	}
	return m
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

	cs := fixedConsensusStateDummy()

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
		lineNum := thisCase.proposeLine
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
		lineNum := thisCase.prevoteLine
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
		lineNum := thisCase.precommitLine
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
