package consensus

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/internal/eventbus"
	tmpubsub "github.com/tendermint/tendermint/internal/pubsub"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	tmtimemocks "github.com/tendermint/tendermint/libs/time/mocks"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

const (
	// blockTimeIota is used in the test harness as the time between
	// blocks when not otherwise specified.
	blockTimeIota = time.Millisecond
)

// pbtsTestHarness constructs a Tendermint network that can be used for testing the
// implementation of the Proposer-Based timestamps algorithm.
// It runs a series of consensus heights and captures timing of votes and events.
type pbtsTestHarness struct {
	// configuration options set by the user of the test harness.
	pbtsTestConfiguration

	// The timestamp of the first block produced by the network.
	firstBlockTime time.Time

	// The Tendermint consensus state machine being run during
	// a run of the pbtsTestHarness.
	observedState *State

	// A stub for signing votes and messages using the key
	// from the observedState.
	observedValidator *validatorStub

	// A list of simulated validators that interact with the observedState and are
	// fully controlled by the test harness.
	otherValidators []*validatorStub

	// The mock time source used by all of the validator stubs in the test harness.
	// This mock clock allows the test harness to produce votes and blocks with arbitrary
	// timestamps.
	validatorClock *tmtimemocks.Source

	chainID string

	// channels for verifying that the observed validator completes certain actions.
	ensureProposalCh, roundCh, blockCh, ensureVoteCh <-chan tmpubsub.Message

	// channel of events from the observed validator annotated with the timestamp
	// the event was received.
	eventCh <-chan timestampedEvent

	currentHeight int64
	currentRound  int32
}

type pbtsTestConfiguration struct {
	// The timestamp consensus parameters to be used by the state machine under test.
	synchronyParams types.SynchronyParams

	// The setting to use for the TimeoutPropose configuration parameter.
	timeoutPropose time.Duration

	// The genesis time
	genesisTime time.Time

	// The times offset from height 1 block time of the block proposed at height 2.
	height2ProposedBlockOffset time.Duration

	// The time offset from height 1 block time at which the proposal at height 2 should be delivered.
	height2ProposalTimeDeliveryOffset time.Duration

	// The time offset from height 1 block time of the block proposed at height 4.
	// At height 4, the proposed block and the deliver offsets are the same so
	// that timely-ness does not affect height 4.
	height4ProposedBlockOffset time.Duration
}

func newPBTSTestHarness(ctx context.Context, t *testing.T, tc pbtsTestConfiguration) pbtsTestHarness {
	t.Helper()
	const validators = 4
	cfg := configSetup(t)
	clock := new(tmtimemocks.Source)

	if tc.genesisTime.IsZero() {
		tc.genesisTime = time.Now()
	}

	if tc.height4ProposedBlockOffset == 0 {

		// Set a default height4ProposedBlockOffset.
		// Use a proposed block time that is greater than the time that the
		// block at height 2 was delivered. Height 3 is not relevant for testing
		// and always occurs blockTimeIota before height 4. If not otherwise specified,
		// height 4 therefore occurs 2*blockTimeIota after height 2.
		tc.height4ProposedBlockOffset = tc.height2ProposalTimeDeliveryOffset + 2*blockTimeIota
	}
	consensusParams := factory.ConsensusParams()
	consensusParams.Timeout.Propose = tc.timeoutPropose
	consensusParams.Synchrony = tc.synchronyParams

	state, privVals := makeGenesisState(ctx, t, cfg, genesisStateArgs{
		Params:     consensusParams,
		Time:       tc.genesisTime,
		Validators: validators,
	})
	cs := newState(ctx, t, log.NewNopLogger(), state, privVals[0], kvstore.NewApplication())
	vss := make([]*validatorStub, validators)
	for i := 0; i < validators; i++ {
		vss[i] = newValidatorStub(privVals[i], int32(i))
	}
	incrementHeight(vss[1:]...)

	for _, vs := range vss {
		vs.clock = clock
	}
	pubKey, err := vss[0].PrivValidator.GetPubKey(ctx)
	require.NoError(t, err)

	eventCh := timestampedCollector(ctx, t, cs.eventBus)

	return pbtsTestHarness{
		pbtsTestConfiguration: tc,
		observedValidator:     vss[0],
		observedState:         cs,
		otherValidators:       vss[1:],
		validatorClock:        clock,
		currentHeight:         1,
		chainID:               cfg.ChainID(),
		roundCh:               subscribe(ctx, t, cs.eventBus, types.EventQueryNewRound),
		ensureProposalCh:      subscribe(ctx, t, cs.eventBus, types.EventQueryCompleteProposal),
		blockCh:               subscribe(ctx, t, cs.eventBus, types.EventQueryNewBlock),
		ensureVoteCh:          subscribeToVoterBuffered(ctx, t, cs, pubKey.Address()),
		eventCh:               eventCh,
	}
}

func (p *pbtsTestHarness) observedValidatorProposerHeight(ctx context.Context, t *testing.T, previousBlockTime time.Time) (heightResult, time.Time) {
	p.validatorClock.On("Now").Return(p.genesisTime.Add(p.height2ProposedBlockOffset)).Times(6)

	ensureNewRound(t, p.roundCh, p.currentHeight, p.currentRound)

	timeout := time.Until(previousBlockTime.Add(ensureTimeout))
	ensureProposalWithTimeout(t, p.ensureProposalCh, p.currentHeight, p.currentRound, nil, timeout)

	rs := p.observedState.GetRoundState()
	bid := types.BlockID{Hash: rs.ProposalBlock.Hash(), PartSetHeader: rs.ProposalBlockParts.Header()}
	ensurePrevote(t, p.ensureVoteCh, p.currentHeight, p.currentRound)
	signAddVotes(ctx, t, p.observedState, tmproto.PrevoteType, p.chainID, bid, p.otherValidators...)

	signAddVotes(ctx, t, p.observedState, tmproto.PrecommitType, p.chainID, bid, p.otherValidators...)
	ensurePrecommit(t, p.ensureVoteCh, p.currentHeight, p.currentRound)

	ensureNewBlock(t, p.blockCh, p.currentHeight)

	vk, err := p.observedValidator.GetPubKey(ctx)
	require.NoError(t, err)
	res := collectHeightResults(ctx, t, p.eventCh, p.currentHeight, vk.Address())

	p.currentHeight++
	incrementHeight(p.otherValidators...)
	return res, rs.ProposalBlock.Time
}

func (p *pbtsTestHarness) height2(ctx context.Context, t *testing.T) heightResult {
	signer := p.otherValidators[0].PrivValidator
	return p.nextHeight(ctx, t, signer,
		p.firstBlockTime.Add(p.height2ProposalTimeDeliveryOffset),
		p.firstBlockTime.Add(p.height2ProposedBlockOffset),
		p.firstBlockTime.Add(p.height2ProposedBlockOffset+10*blockTimeIota))
}

func (p *pbtsTestHarness) intermediateHeights(ctx context.Context, t *testing.T) {
	signer := p.otherValidators[1].PrivValidator
	p.nextHeight(ctx, t, signer,
		p.firstBlockTime.Add(p.height2ProposedBlockOffset+10*blockTimeIota),
		p.firstBlockTime.Add(p.height2ProposedBlockOffset+10*blockTimeIota),
		p.firstBlockTime.Add(p.height4ProposedBlockOffset))

	signer = p.otherValidators[2].PrivValidator
	p.nextHeight(ctx, t, signer,
		p.firstBlockTime.Add(p.height4ProposedBlockOffset),
		p.firstBlockTime.Add(p.height4ProposedBlockOffset),
		time.Now())
}

func (p *pbtsTestHarness) height5(ctx context.Context, t *testing.T) (heightResult, time.Time) {
	return p.observedValidatorProposerHeight(ctx, t, p.firstBlockTime.Add(p.height4ProposedBlockOffset))
}

func (p *pbtsTestHarness) nextHeight(ctx context.Context, t *testing.T, proposer types.PrivValidator, deliverTime, proposedTime, nextProposedTime time.Time) heightResult {
	p.validatorClock.On("Now").Return(nextProposedTime).Times(6)

	ensureNewRound(t, p.roundCh, p.currentHeight, p.currentRound)

	b, err := p.observedState.createProposalBlock(ctx)
	require.NoError(t, err)
	b.Height = p.currentHeight
	b.Header.Height = p.currentHeight
	b.Header.Time = proposedTime

	k, err := proposer.GetPubKey(ctx)
	require.NoError(t, err)
	b.Header.ProposerAddress = k.Address()
	ps, err := b.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)
	bid := types.BlockID{Hash: b.Hash(), PartSetHeader: ps.Header()}
	prop := types.NewProposal(p.currentHeight, 0, -1, bid, proposedTime)
	tp := prop.ToProto()

	if err := proposer.SignProposal(ctx, p.observedState.state.ChainID, tp); err != nil {
		t.Fatalf("error signing proposal: %s", err)
	}

	time.Sleep(time.Until(deliverTime))
	prop.Signature = tp.Signature
	if err := p.observedState.SetProposalAndBlock(ctx, prop, b, ps, "peerID"); err != nil {
		t.Fatal(err)
	}
	ensureProposal(t, p.ensureProposalCh, p.currentHeight, 0, bid)

	ensurePrevote(t, p.ensureVoteCh, p.currentHeight, p.currentRound)
	signAddVotes(ctx, t, p.observedState, tmproto.PrevoteType, p.chainID, bid, p.otherValidators...)

	signAddVotes(ctx, t, p.observedState, tmproto.PrecommitType, p.chainID, bid, p.otherValidators...)
	ensurePrecommit(t, p.ensureVoteCh, p.currentHeight, p.currentRound)

	vk, err := p.observedValidator.GetPubKey(ctx)
	require.NoError(t, err)
	res := collectHeightResults(ctx, t, p.eventCh, p.currentHeight, vk.Address())
	ensureNewBlock(t, p.blockCh, p.currentHeight)

	p.currentHeight++
	incrementHeight(p.otherValidators...)
	return res
}

func timestampedCollector(ctx context.Context, t *testing.T, eb *eventbus.EventBus) <-chan timestampedEvent {
	t.Helper()

	// Since eventCh is not read until the end of each height, it must be large
	// enough to hold all of the events produced during a single height.
	eventCh := make(chan timestampedEvent, 100)

	if err := eb.Observe(ctx, func(msg tmpubsub.Message) error {
		eventCh <- timestampedEvent{
			ts: time.Now(),
			m:  msg,
		}
		return nil
	}, types.EventQueryVote, types.EventQueryCompleteProposal); err != nil {
		t.Fatalf("Failed to observe query %v: %v", types.EventQueryVote, err)
	}
	return eventCh
}

func collectHeightResults(ctx context.Context, t *testing.T, eventCh <-chan timestampedEvent, height int64, address []byte) heightResult {
	t.Helper()
	var res heightResult
	for event := range eventCh {
		switch v := event.m.Data().(type) {
		case types.EventDataVote:
			if v.Vote.Height > height {
				t.Fatalf("received prevote from unexpected height, expected: %d, saw: %d", height, v.Vote.Height)
			}
			if !bytes.Equal(address, v.Vote.ValidatorAddress) {
				continue
			}
			if v.Vote.Type != tmproto.PrevoteType {
				continue
			}
			res.prevote = v.Vote
			res.prevoteIssuedAt = event.ts

		case types.EventDataCompleteProposal:
			if v.Height > height {
				t.Fatalf("received proposal from unexpected height, expected: %d, saw: %d", height, v.Height)
			}
			res.proposalIssuedAt = event.ts
		}
		if res.isComplete() {
			return res
		}
	}
	t.Fatalf("complete height result never seen for height %d", height)

	panic("unreachable")
}

type timestampedEvent struct {
	ts time.Time
	m  tmpubsub.Message
}

func (p *pbtsTestHarness) run(ctx context.Context, t *testing.T) resultSet {
	startTestRound(ctx, p.observedState, p.currentHeight, p.currentRound)

	r1, proposalBlockTime := p.observedValidatorProposerHeight(ctx, t, p.genesisTime)
	p.firstBlockTime = proposalBlockTime
	r2 := p.height2(ctx, t)
	p.intermediateHeights(ctx, t)
	r5, _ := p.height5(ctx, t)
	return resultSet{
		genesisHeight: r1,
		height2:       r2,
		height5:       r5,
	}
}

type resultSet struct {
	genesisHeight heightResult
	height2       heightResult
	height5       heightResult
}

type heightResult struct {
	proposalIssuedAt time.Time
	prevote          *types.Vote
	prevoteIssuedAt  time.Time
}

func (hr heightResult) isComplete() bool {
	return !hr.proposalIssuedAt.IsZero() && !hr.prevoteIssuedAt.IsZero() && hr.prevote != nil
}

// TestProposerWaitsForGenesisTime tests that a proposer will not propose a block
// until after the genesis time has passed. The test sets the genesis time in the
// future and then ensures that the observed validator waits to propose a block.
func TestProposerWaitsForGenesisTime(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create a genesis time far (enough) in the future.
	initialTime := time.Now().Add(800 * time.Millisecond)
	cfg := pbtsTestConfiguration{
		synchronyParams: types.SynchronyParams{
			Precision:    10 * time.Millisecond,
			MessageDelay: 10 * time.Millisecond,
		},
		timeoutPropose:                    10 * time.Millisecond,
		genesisTime:                       initialTime,
		height2ProposalTimeDeliveryOffset: 10 * time.Millisecond,
		height2ProposedBlockOffset:        10 * time.Millisecond,
		height4ProposedBlockOffset:        30 * time.Millisecond,
	}

	pbtsTest := newPBTSTestHarness(ctx, t, cfg)
	results := pbtsTest.run(ctx, t)

	// ensure that the proposal was issued after the genesis time.
	assert.True(t, results.genesisHeight.proposalIssuedAt.After(cfg.genesisTime))
}

// TestProposerWaitsForPreviousBlock tests that the proposer of a block waits until
// the block time of the previous height has passed to propose the next block.
// The test harness ensures that the observed validator will be the proposer at
// height 1 and height 5. The test sets the block time of height 4 in the future
// and then verifies that the observed validator waits until after the block time
// of height 4 to propose a block at height 5.
func TestProposerWaitsForPreviousBlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	initialTime := time.Now().Add(time.Millisecond * 50)
	cfg := pbtsTestConfiguration{
		synchronyParams: types.SynchronyParams{
			Precision:    100 * time.Millisecond,
			MessageDelay: 500 * time.Millisecond,
		},
		timeoutPropose:                    50 * time.Millisecond,
		genesisTime:                       initialTime,
		height2ProposalTimeDeliveryOffset: 150 * time.Millisecond,
		height2ProposedBlockOffset:        100 * time.Millisecond,
		height4ProposedBlockOffset:        800 * time.Millisecond,
	}

	pbtsTest := newPBTSTestHarness(ctx, t, cfg)
	results := pbtsTest.run(ctx, t)

	// the observed validator is the proposer at height 5.
	// ensure that the observed validator did not propose a block until after
	// the time configured for height 4.
	assert.True(t, results.height5.proposalIssuedAt.After(pbtsTest.firstBlockTime.Add(cfg.height4ProposedBlockOffset)))

	// Ensure that the validator issued a prevote for a non-nil block.
	assert.NotNil(t, results.height5.prevote.BlockID.Hash)
}

func TestProposerWaitTime(t *testing.T) {
	genesisTime, err := time.Parse(time.RFC3339, "2019-03-13T23:00:00Z")
	require.NoError(t, err)
	testCases := []struct {
		name              string
		previousBlockTime time.Time
		localTime         time.Time
		expectedWait      time.Duration
	}{
		{
			name:              "block time greater than local time",
			previousBlockTime: genesisTime.Add(5 * time.Nanosecond),
			localTime:         genesisTime.Add(1 * time.Nanosecond),
			expectedWait:      4 * time.Nanosecond,
		},
		{
			name:              "local time greater than block time",
			previousBlockTime: genesisTime.Add(1 * time.Nanosecond),
			localTime:         genesisTime.Add(5 * time.Nanosecond),
			expectedWait:      0,
		},
		{
			name:              "both times equal",
			previousBlockTime: genesisTime.Add(5 * time.Nanosecond),
			localTime:         genesisTime.Add(5 * time.Nanosecond),
			expectedWait:      0,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			mockSource := new(tmtimemocks.Source)
			mockSource.On("Now").Return(testCase.localTime)

			ti := proposerWaitTime(mockSource, testCase.previousBlockTime)
			assert.Equal(t, testCase.expectedWait, ti)
		})
	}
}

func TestTimelyProposal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	initialTime := time.Now()

	cfg := pbtsTestConfiguration{
		synchronyParams: types.SynchronyParams{
			Precision:    10 * time.Millisecond,
			MessageDelay: 140 * time.Millisecond,
		},
		timeoutPropose:                    40 * time.Millisecond,
		genesisTime:                       initialTime,
		height2ProposedBlockOffset:        15 * time.Millisecond,
		height2ProposalTimeDeliveryOffset: 30 * time.Millisecond,
	}

	pbtsTest := newPBTSTestHarness(ctx, t, cfg)
	results := pbtsTest.run(ctx, t)
	require.NotNil(t, results.height2.prevote.BlockID.Hash)
}

func TestTooFarInThePastProposal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// localtime > proposedBlockTime + MsgDelay + Precision
	cfg := pbtsTestConfiguration{
		synchronyParams: types.SynchronyParams{
			Precision:    1 * time.Millisecond,
			MessageDelay: 10 * time.Millisecond,
		},
		timeoutPropose:                    50 * time.Millisecond,
		height2ProposedBlockOffset:        15 * time.Millisecond,
		height2ProposalTimeDeliveryOffset: 27 * time.Millisecond,
	}

	pbtsTest := newPBTSTestHarness(ctx, t, cfg)
	results := pbtsTest.run(ctx, t)

	require.Nil(t, results.height2.prevote.BlockID.Hash)
}

func TestTooFarInTheFutureProposal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// localtime < proposedBlockTime - Precision
	cfg := pbtsTestConfiguration{
		synchronyParams: types.SynchronyParams{
			Precision:    1 * time.Millisecond,
			MessageDelay: 10 * time.Millisecond,
		},
		timeoutPropose:                    50 * time.Millisecond,
		height2ProposedBlockOffset:        100 * time.Millisecond,
		height2ProposalTimeDeliveryOffset: 10 * time.Millisecond,
		height4ProposedBlockOffset:        150 * time.Millisecond,
	}

	pbtsTest := newPBTSTestHarness(ctx, t, cfg)
	results := pbtsTest.run(ctx, t)

	require.Nil(t, results.height2.prevote.BlockID.Hash)
}
