package sanity

import (
	"errors"
	"time"

	"github.com/ds-test-framework/scheduler/testlib"
	smlib "github.com/ds-test-framework/scheduler/testlib/statemachine"
	"github.com/ds-test-framework/scheduler/types"
	"github.com/ds-test-framework/tendermint-test/util"
)

type testCaseOneVoteCount struct {
	// keeps track of the prevoted replicas for a given blockID
	recorded map[string]map[string]bool
	// keeps track of how many votes are delivered to the replicas
	// first key is for vote type and second key is for replica type
	delivered map[string]map[string]int
}

func newTestCaseOneVoteCount() *testCaseOneVoteCount {
	return &testCaseOneVoteCount{
		recorded:  make(map[string]map[string]bool),
		delivered: make(map[string]map[string]int),
	}
}
func getTestCaseOneVoteCount(c *testlib.Context, round int) *testCaseOneVoteCount {
	voteCountI, _ := c.Vars.Get("voteCount")
	voteCount := voteCountI.(map[int]*testCaseOneVoteCount)
	_, ok := voteCount[round]
	if !ok {
		voteCount[round] = newTestCaseOneVoteCount()
	}
	return voteCount[round]
}

func setTestCaseOneVoteCount(c *testlib.Context, v *testCaseOneVoteCount, round int) {
	voteCountI, _ := c.Vars.Get("voteCount")
	voteCount := voteCountI.(map[int]*testCaseOneVoteCount)

	voteCount[round] = v
	c.Vars.Set("voteCount", voteCount)
}

func testCaseOneSetup(c *testlib.Context) error {
	faults := int((c.Replicas.Cap() - 1) / 3)
	partitioner := util.NewStaticPartitioner(c.Replicas, faults)
	partitioner.NewPartition(0)
	partition, _ := partitioner.GetPartition(0)
	c.Vars.Set("partition", partition)
	c.Vars.Set("faults", faults)
	c.Vars.Set("voteCount", make(map[int]*testCaseOneVoteCount))
	return nil
}

func getPartition(c *testlib.Context) *util.Partition {
	partition, _ := c.Vars.Get("partition")
	return partition.(*util.Partition)
}

type testCaseOneFilters struct{}

func (testCaseOneFilters) faultyFilter(c *smlib.Context) ([]*types.Message, bool) {

	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return []*types.Message{}, false
	}
	if tMsg.Type != util.Prevote && tMsg.Type != util.Precommit {
		return []*types.Message{}, false
	}

	faulty, _ := getPartition(c.Context).GetPart("faulty")

	if faulty.Contains(tMsg.From) {
		replica, _ := c.Replicas.Get(tMsg.From)
		newVote, err := util.ChangeVoteToNil(replica, tMsg)
		if err != nil {
			return []*types.Message{tMsg.SchedulerMessage}, true
		}
		newMsgB, err := util.Marshal(newVote)
		if err != nil {
			return []*types.Message{tMsg.SchedulerMessage}, true
		}
		return []*types.Message{c.NewMessage(tMsg.SchedulerMessage, newMsgB)}, true
		// } else if faulty.Contains(tMsg.To) {
		// 	// Deliver everything to faulty because we need it to advance!
		// 	return []*types.Message{tMsg.SchedulerMessage}, true
	}
	return []*types.Message{}, false
}

func (testCaseOneFilters) round0(c *smlib.Context) ([]*types.Message, bool) {
	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return []*types.Message{}, false
	}
	_, round := util.ExtractHR(tMsg)

	// 1. Only keep count of messages to deliver in round 0
	if round != 0 {
		return []*types.Message{tMsg.SchedulerMessage}, true
	}

	if tMsg.Type != util.Precommit {
		return []*types.Message{tMsg.SchedulerMessage}, true
	}

	votes := getTestCaseOneVoteCount(c.Context, round)
	faulty, _ := getPartition(c.Context).GetPart("faulty")

	faults, _ := c.Vars.GetInt("faults")
	_, ok := votes.delivered[string(tMsg.Type)]
	if !ok {
		votes.delivered[string(tMsg.Type)] = make(map[string]int)
	}
	delivered := votes.delivered[string(tMsg.Type)]
	_, ok = delivered[string(tMsg.To)]
	if !ok {
		delivered[string(tMsg.To)] = 0
	}
	curDelivered := delivered[string(tMsg.To)]
	if faulty.Contains(tMsg.To) {
		// deliver only 2f-1 votes so that it does not commit.
		// In total it receives 2f votes and hence does not make progress until it hears 2f+1 votes of the next round
		if curDelivered < 2*faults-1 {
			votes.delivered[string(tMsg.Type)][string(tMsg.To)] = curDelivered + 1
			setTestCaseOneVoteCount(c.Context, votes, round)
			return []*types.Message{tMsg.SchedulerMessage}, true
		}
	} else if curDelivered <= 2*faults-1 {
		votes.delivered[string(tMsg.Type)][string(tMsg.To)] = curDelivered + 1
		setTestCaseOneVoteCount(c.Context, votes, round)
		return []*types.Message{tMsg.SchedulerMessage}, true
	}

	return []*types.Message{}, true
}

type testCaseOneCond struct{}

func (t testCaseOneCond) quorumCond(c *smlib.Context) bool {

	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return false
	}
	faulty, _ := getPartition(c.Context).GetPart("faulty")
	faults, _ := c.Vars.GetInt("faults")
	if faulty.Contains(tMsg.From) {
		return false
	}

	if tMsg.Type != util.Prevote {
		return false
	}
	_, round := util.ExtractHR(tMsg)
	blockID, _ := util.GetVoteBlockIDS(tMsg)

	votes := getTestCaseOneVoteCount(c.Context, round)
	_, ok := votes.recorded[blockID]
	if !ok {
		votes.recorded[blockID] = make(map[string]bool)
	}
	votes.recorded[blockID][string(tMsg.From)] = true
	setTestCaseOneVoteCount(c.Context, votes, round)

	if round != 0 {
		roundZeroVotes := getTestCaseOneVoteCount(c.Context, 0)

		ok, err = t.findIntersection(votes.recorded, roundZeroVotes.recorded, faults)
		if err == errDifferentQuorum {
			c.Abort()
		}
		if ok {
			c.Vars.Set("QuorumIntersection", true)
		}
		return ok
	}
	return false
}

var (
	errDifferentQuorum = errors.New("different proposal")
)

func (testCaseOneCond) findIntersection(new, old map[string]map[string]bool, faults int) (bool, error) {
	quorumProposal := ""
	quorum := make(map[string]bool)
	for k, v := range new {
		if len(v) >= 2*faults+1 {
			quorumProposal = k
			for replica := range v {
				quorum[replica] = true
			}
			break
		}
	}
	oldQuorum, ok := old[quorumProposal]
	if !ok {
		return false, errDifferentQuorum
	}
	intersection := 0
	for replica := range oldQuorum {
		_, ok := quorum[replica]
		if ok {
			intersection++
			if intersection > faults {
				return true, nil
			}
		}
	}

	return false, nil
}

// States:
// 	1. Skip rounds by not delivering enough precommits to the replicas
// 		1.1. Ensure one faulty replica prevotes and precommits nil
// 	2. Check that in the new round there is a quorum intersection of f+1
// 		2.1 Record the votes on the proposal to check for quorum intersection (Proposal should be same in both rounds)
func OneTestCase() *testlib.TestCase {
	cond := testCaseOneCond{}
	filters := testCaseOneFilters{}

	sm := smlib.NewStateMachine()
	sm.Builder().On(cond.quorumCond, smlib.SuccessStateLabel)

	handler := smlib.NewAsyncStateMachineHandler(sm)
	handler.AddEventHandler(filters.faultyFilter)
	handler.AddEventHandler(filters.round0)

	testcase := testlib.NewTestCase("QuorumIntersection", 50*time.Second, handler)
	testcase.SetupFunc(testCaseOneSetup)
	testcase.AssertFn(func(c *testlib.Context) bool {
		i, ok := c.Vars.GetBool("QuorumIntersection")
		return ok && i
	})

	return testcase
}
