package consensus

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/tendermint/tendermint/config/tendermint_test"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
)

func init() {
	config = tendermint_test.ResetConfig("consensus_replay_test")
}

// TODO: these tests ensure we can always recover from any state of the wal,
// assuming it comes with a correct related state for the priv_validator.json.
// It would be better to verify explicitly which states we can recover from without the wal
// and which ones we need the wal for - then we'd also be able to only flush the
// wal writer when we need to, instead of with every message.

var data_dir = path.Join(GoPath, "src/github.com/tendermint/tendermint/consensus", "test_data")

// the priv validator changes step at these lines for a block with 1 val and 1 part
var baseStepChanges = []int{3, 6, 8}

// test recovery from each line in each testCase
var testCases = []*testCase{
	newTestCase("empty_block", baseStepChanges),   // empty block (has 1 block part)
	newTestCase("small_block1", baseStepChanges),  // small block with txs in 1 block part
	newTestCase("small_block2", []int{3, 10, 12}), // small block with txs across 5 smaller block parts
}

type testCase struct {
	name    string
	log     string       //full cs wal
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

func writeWAL(walMsgs string) string {
	tempDir := os.TempDir()
	walDir := tempDir + "/wal" + RandStr(12)
	// Create WAL directory
	err := EnsureDir(walDir, 0700)
	if err != nil {
		panic(err)
	}
	// Write the needed WAL to file
	err = WriteFile(walDir+"/wal", []byte(walMsgs), 0600)
	if err != nil {
		panic(err)
	}
	return walDir
}

func waitForBlock(newBlockCh chan interface{}, thisCase *testCase, i int) {
	after := time.After(time.Second * 10)
	select {
	case <-newBlockCh:
	case <-after:
		panic(Fmt("Timed out waiting for new block for case '%s' line %d", thisCase.name, i))
	}
}

func runReplayTest(t *testing.T, cs *ConsensusState, walDir string, newBlockCh chan interface{},
	thisCase *testCase, i int) {

	cs.config.Set("cs_wal_dir", walDir)
	cs.Start()
	// Wait to make a new block.
	// This is just a signal that we haven't halted; its not something contained in the WAL itself.
	// Assuming the consensus state is running, replay of any WAL, including the empty one,
	// should eventually be followed by a new block, or else something is wrong
	waitForBlock(newBlockCh, thisCase, i)
	cs.evsw.Stop()
	cs.Stop()
LOOP:
	for {
		select {
		case <-newBlockCh:
		default:
			break LOOP
		}
	}
	cs.Wait()
}

func toPV(pv PrivValidator) *types.PrivValidator {
	return pv.(*types.PrivValidator)
}

func setupReplayTest(thisCase *testCase, nLines int, crashAfter bool) (*ConsensusState, chan interface{}, string, string) {
	fmt.Println("-------------------------------------")
	log.Notice(Fmt("Starting replay test %v (of %d lines of WAL). Crash after = %v", thisCase.name, nLines, crashAfter))

	lineStep := nLines
	if crashAfter {
		lineStep -= 1
	}

	split := strings.Split(thisCase.log, "\n")
	lastMsg := split[nLines]

	// we write those lines up to (not including) one with the signature
	walDir := writeWAL(strings.Join(split[:nLines], "\n") + "\n")

	cs := fixedConsensusStateDummy()

	// set the last step according to when we crashed vs the wal
	toPV(cs.privValidator).LastHeight = 1 // first block
	toPV(cs.privValidator).LastStep = thisCase.stepMap[lineStep]

	log.Warn("setupReplayTest", "LastStep", toPV(cs.privValidator).LastStep)

	newBlockCh := subscribeToEvent(cs.evsw, "tester", types.EventStringNewBlock(), 1)

	return cs, newBlockCh, lastMsg, walDir
}

func readTimedWALMessage(t *testing.T, walMsg string) TimedWALMessage {
	var err error
	var msg TimedWALMessage
	wire.ReadJSON(&msg, []byte(walMsg), &err)
	if err != nil {
		t.Fatalf("Error reading json data: %v", err)
	}
	return msg
}

//-----------------------------------------------
// Test the log at every iteration, and set the privVal last step
// as if the log was written after signing, before the crash

func TestReplayCrashAfterWrite(t *testing.T) {
	for _, thisCase := range testCases {
		split := strings.Split(thisCase.log, "\n")
		for i := 0; i < len(split)-1; i++ {
			cs, newBlockCh, _, walDir := setupReplayTest(thisCase, i+1, true)
			runReplayTest(t, cs, walDir, newBlockCh, thisCase, i+1)
		}
	}
}

//-----------------------------------------------
// Test the log as if we crashed after signing but before writing.
// This relies on privValidator.LastSignature being set

func TestReplayCrashBeforeWritePropose(t *testing.T) {
	for _, thisCase := range testCases {
		lineNum := thisCase.proposeLine
		// setup replay test where last message is a proposal
		cs, newBlockCh, proposalMsg, walDir := setupReplayTest(thisCase, lineNum, false)
		msg := readTimedWALMessage(t, proposalMsg)
		proposal := msg.Msg.(msgInfo).Msg.(*ProposalMessage)
		// Set LastSig
		toPV(cs.privValidator).LastSignBytes = types.SignBytes(cs.state.ChainID, proposal.Proposal)
		toPV(cs.privValidator).LastSignature = proposal.Proposal.Signature
		runReplayTest(t, cs, walDir, newBlockCh, thisCase, lineNum)
	}
}

func TestReplayCrashBeforeWritePrevote(t *testing.T) {
	for _, thisCase := range testCases {
		testReplayCrashBeforeWriteVote(t, thisCase, thisCase.prevoteLine, types.EventStringCompleteProposal())
	}
}

func TestReplayCrashBeforeWritePrecommit(t *testing.T) {
	for _, thisCase := range testCases {
		testReplayCrashBeforeWriteVote(t, thisCase, thisCase.precommitLine, types.EventStringPolka())
	}
}

func testReplayCrashBeforeWriteVote(t *testing.T, thisCase *testCase, lineNum int, eventString string) {
	// setup replay test where last message is a vote
	cs, newBlockCh, voteMsg, walDir := setupReplayTest(thisCase, lineNum, false)
	types.AddListenerForEvent(cs.evsw, "tester", eventString, func(data types.TMEventData) {
		msg := readTimedWALMessage(t, voteMsg)
		vote := msg.Msg.(msgInfo).Msg.(*VoteMessage)
		// Set LastSig
		toPV(cs.privValidator).LastSignBytes = types.SignBytes(cs.state.ChainID, vote.Vote)
		toPV(cs.privValidator).LastSignature = vote.Vote.Signature
	})
	runReplayTest(t, cs, walDir, newBlockCh, thisCase, lineNum)
}
