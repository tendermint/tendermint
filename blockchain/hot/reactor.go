package hot

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/blockchain"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

const (
	// HotBlockchainChannel is a channel for blocks/commits and status updates under hot sync mode.
	HotBlockchainChannel = byte(0x41)

	messageQueueSize = 1000

	switchToConsensusIntervalSeconds = 1

	maxSubscribeBlocks = 50

	// NOTE: keep up to date with blockCommitResponseMessage
	blockCommitResponseMessagePrefixSize = 4
	blockCommitMessageFieldKeySize       = 2
	// the size of commit is dynamic, assume no more than 1M.
	commitSizeBytes = 1024 * 1024
	maxMsgSize      = types.MaxBlockSizeBytes +
		blockCommitResponseMessagePrefixSize +
		blockCommitMessageFieldKeySize +
		commitSizeBytes
)

type consensusReactor interface {
	// for when we switch from hotBlockchain reactor and hot sync to
	// the consensus machine
	SwitchToConsensus(sm.State, int)
}

// BlockchainReactor handles low latency catchup when there is small lag.
type BlockchainReactor struct {
	p2p.BaseReactor

	pool          *BlockPool
	privValidator types.PrivValidator

	messagesCh <-chan Message
}

type BlockChainOption func(*BlockchainReactor)

// NewBlockChainReactor returns new reactor instance.
func NewBlockChainReactor(state sm.State, blockExec *sm.BlockExecutor, store *blockchain.BlockStore, hotSync, fastSync bool, blockTimeout time.Duration, options ...BlockChainOption) *BlockchainReactor {

	if state.LastBlockHeight != store.Height() {
		panic(fmt.Sprintf("state (%v) and store (%v) height mismatch", state.LastBlockHeight,
			store.Height()))
	}

	messagesCh := make(chan Message, messageQueueSize)
	var st SyncPattern
	if fastSync {
		st = Mute
	} else if hotSync {
		st = Hot
	} else {
		st = Consensus
	}
	pool := NewBlockPool(store, blockExec, state, messagesCh, st, blockTimeout)

	hbcR := &BlockchainReactor{
		messagesCh: messagesCh,
		pool:       pool,
	}

	for _, option := range options {
		option(hbcR)
	}

	hbcR.BaseReactor = *p2p.NewBaseReactor("HotSyncBlockChainReactor", hbcR)
	return hbcR
}

// WithMetrics sets the metrics.
func WithMetrics(metrics *Metrics) BlockChainOption {
	return func(hbcR *BlockchainReactor) { hbcR.pool.setMetrics(metrics) }
}

// WithMetrics sets the metrics.
func WithEventBus(eventBs *types.EventBus) BlockChainOption {
	return func(hbcR *BlockchainReactor) { hbcR.pool.setEventBus(eventBs) }
}

func (hbcR *BlockchainReactor) SetPrivValidator(priv types.PrivValidator) {
	hbcR.privValidator = priv
}

// SetLogger implements cmn.Service by setting the logger on reactor and pool.
func (hbcR *BlockchainReactor) SetLogger(l log.Logger) {
	hbcR.BaseService.Logger = l
	hbcR.pool.SetLogger(l)
}

// OnStart implements cmn.Service.
func (hbcR *BlockchainReactor) OnStart() error {
	err := hbcR.pool.Start()
	if err != nil {
		return err
	}
	go hbcR.poolRoutine()
	return nil
}

// OnStop implements cmn.Service.
func (hbcR *BlockchainReactor) OnStop() {
	hbcR.pool.Stop()
}

// GetChannels implements Reactor
func (hbcR *BlockchainReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:                  HotBlockchainChannel,
			Priority:            10,
			SendQueueCapacity:   1000,
			RecvBufferCapacity:  50 * 4096,
			RecvMessageCapacity: maxMsgSize,
		},
	}
}

func (hbcR *BlockchainReactor) AddPeer(peer p2p.Peer) {
	hbcR.pool.AddPeer(peer)
}

func (hbcR *BlockchainReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	hbcR.pool.RemovePeer(peer)
}

// Receive implements Reactor by handling 5 types of messages (look below).
func (hbcR *BlockchainReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	msg, err := decodeMsg(msgBytes)
	if err != nil {
		hbcR.Logger.Error("Error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		hbcR.Switch.StopPeerForError(src, err)
		return
	}

	if err = msg.ValidateBasic(); err != nil {
		hbcR.Logger.Error("Peer sent us invalid msg", "peer", src, "msg", msg, "err", err)
		hbcR.Switch.StopPeerForError(src, err)
		return
	}
	switch msg := msg.(type) {
	case *blockSubscribeMessage:
		hbcR.Logger.Debug("receive blockSubscribeMessage from peer", "peer", src.ID(), "from_height", msg.FromHeight, "to_height", msg.ToHeight)
		hbcR.pool.handleSubscribeBlock(msg.FromHeight, msg.ToHeight, src)
	case *blockCommitResponseMessage:
		hbcR.Logger.Debug("receive blockCommitResponseMessage from peer", "peer", src.ID(), "height", msg.Height())
		hbcR.pool.handleBlockCommit(msg.Block, msg.Commit, src)
	case *noBlockResponseMessage:
		hbcR.Logger.Debug("receive noBlockResponseMessage from peer", "peer", src.ID(), "height", msg.Height)
		hbcR.pool.handleNoBlock(msg.Height, src)
	case *blockUnSubscribeMessage:
		hbcR.Logger.Debug("receive blockUnSubscribeMessage from peer", "peer", src.ID(), "height", msg.Height)
		hbcR.pool.handleUnSubscribeBlock(msg.Height, src)
	default:
		hbcR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

func (hbcR *BlockchainReactor) SetSwitch(sw *p2p.Switch) {
	hbcR.Switch = sw
	hbcR.pool.candidatePool.swh = sw
}

func (hbcR *BlockchainReactor) SwitchToHotSync(state sm.State, blockSynced int32) {
	hbcR.pool.SwitchToHotSync(state, blockSynced)
}

func (hbcR *BlockchainReactor) SwitchToConsensusSync(state sm.State) {
	hbcR.pool.SwitchToConsensusSync(&state)
}

func (hbcR *BlockchainReactor) poolRoutine() {
	switchToConsensusTicker := time.NewTicker(switchToConsensusIntervalSeconds * time.Second)
	defer switchToConsensusTicker.Stop()
	for {
		select {
		case <-hbcR.Quit():
			return
		case <-hbcR.pool.Quit():
			return
		case message := <-hbcR.messagesCh:
			peer := hbcR.Switch.Peers().Get(message.peerId)
			if peer == nil {
				continue
			}
			msgBytes := cdc.MustMarshalBinaryBare(message.blockChainMessage)
			hbcR.Logger.Debug(fmt.Sprintf("send message %s", message.blockChainMessage.String()), "peer", peer.ID())
			queued := peer.TrySend(HotBlockchainChannel, msgBytes)
			if !queued {
				hbcR.Logger.Debug("Send queue is full or no hot sync channel, drop blockChainMessage request", "peer", peer.ID())
			}
		case <-switchToConsensusTicker.C:
			if hbcR.pool.getSyncPattern() == Hot && hbcR.privValidator != nil && hbcR.pool.state.Validators.HasAddress(hbcR.privValidator.GetPubKey().Address()) {
				hbcR.Logger.Info("hot sync switching to consensus sync")
				conR, ok := hbcR.Switch.Reactor("CONSENSUS").(consensusReactor)
				if ok {
					hbcR.pool.SwitchToConsensusSync(nil)
					conR.SwitchToConsensus(hbcR.pool.state, int(hbcR.pool.getBlockSynced()))
				} else {
					// should only happen during testing
				}
			}
		}
	}
}

// BlockchainMessage is a generic message for this reactor.
type BlockchainMessage interface {
	ValidateBasic() error
	String() string
}

//-------------------------------------

// Subscribe block at specified height range
type blockSubscribeMessage struct {
	FromHeight int64
	ToHeight   int64
}

func (m *blockSubscribeMessage) ValidateBasic() error {
	if m.FromHeight < 0 {
		return errors.New("Negative Height")
	}
	if m.ToHeight < m.FromHeight {
		return errors.New("Height is greater than ToHeight")
	}
	if m.ToHeight-m.FromHeight > maxSubscribeBlocks {
		return errors.New("Subscribe too many blocks")
	}
	return nil
}

func (m *blockSubscribeMessage) String() string {
	return fmt.Sprintf("[blockSubscribeMessage from %v, to %v]", m.FromHeight, m.ToHeight)
}

//-------------------------------------

// UnSubscribe block at specified height
// Why not in range?
// Because of peer pick strategy, will not subscribe continuous blocks from peer that considered as unknown or bad.
// And because of reschedule strategy, the previous continuous subscription become discontinuous.
type blockUnSubscribeMessage struct {
	Height int64
}

func (m *blockUnSubscribeMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	return nil
}

func (m *blockUnSubscribeMessage) String() string {
	return fmt.Sprintf("[blockUnSubscribeMessage at %v]", m.Height)
}

//-------------------------------------
type noBlockResponseMessage struct {
	Height int64
}

func (m *noBlockResponseMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	return nil
}

func (m *noBlockResponseMessage) String() string {
	return fmt.Sprintf("[noBlockResponseMessage %v]", m.Height)
}

//-------------------------------------

type blockCommitResponseMessage struct {
	Block  *types.Block
	Commit *types.Commit
}

func (m *blockCommitResponseMessage) Height() int64 {
	// have checked, block and commit can't both be nil
	if m.Block != nil {
		return m.Block.Height
	} else {
		return m.Commit.Height()
	}
}

func (m *blockCommitResponseMessage) ValidateBasic() error {
	if m.Block == nil && m.Commit == nil {
		return errors.New("Both Commit and Block field are missing")
	}

	if m.Commit != nil && m.Block != nil {
		blockId := makeBlockID(m.Block)
		if !blockId.Equals(m.Commit.BlockID) {
			return errors.New("BlockID mismatch")
		}
	}
	if m.Commit != nil {
		if err := m.Commit.ValidateBasic(); err != nil {
			return err
		}
	}
	if m.Block != nil {
		if err := m.Block.ValidateBasic(); err != nil {
			return err
		}
	}
	return nil
}

func (m *blockCommitResponseMessage) String() string {
	return fmt.Sprintf("[blockCommitResponseMessage %v]", m.Height())
}

//-------------------------------------

func RegisterHotBlockchainMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*BlockchainMessage)(nil), nil)
	cdc.RegisterConcrete(&blockSubscribeMessage{}, "tendermint/blockchain/hotBlockSubscribe", nil)
	cdc.RegisterConcrete(&blockUnSubscribeMessage{}, "tendermint/blockchain/hotBlockUnSubscribe", nil)
	cdc.RegisterConcrete(&blockCommitResponseMessage{}, "tendermint/blockchain/hotBlockCommitResponse", nil)
	cdc.RegisterConcrete(&noBlockResponseMessage{}, "tendermint/blockchain/hotNoBlockResponse", nil)
}

func decodeMsg(bz []byte) (msg BlockchainMessage, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("Msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	err = cdc.UnmarshalBinaryBare(bz, &msg)
	return
}
