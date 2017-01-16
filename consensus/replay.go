package consensus

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"

	auto "github.com/tendermint/go-autofile"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"

	"github.com/tendermint/tendermint/types"
)

// Unmarshal and apply a single message to the consensus state
// as if it were received in receiveRoutine
// Lines that start with "#" are ignored.
// NOTE: receiveRoutine should not be running
func (cs *ConsensusState) readReplayMessage(msgBytes []byte, newStepCh chan interface{}) error {
	// Skip over empty and meta lines
	if len(msgBytes) == 0 || msgBytes[0] == '#' {
		return nil
	}
	var err error
	var msg TimedWALMessage
	wire.ReadJSON(&msg, msgBytes, &err)
	if err != nil {
		fmt.Println("MsgBytes:", msgBytes, string(msgBytes))
		return fmt.Errorf("Error reading json data: %v", err)
	}

	// for logging
	switch m := msg.Msg.(type) {
	case types.EventDataRoundState:
		log.Notice("Replay: New Step", "height", m.Height, "round", m.Round, "step", m.Step)
		// these are playback checks
		ticker := time.After(time.Second * 2)
		if newStepCh != nil {
			select {
			case mi := <-newStepCh:
				m2 := mi.(types.EventDataRoundState)
				if m.Height != m2.Height || m.Round != m2.Round || m.Step != m2.Step {
					return fmt.Errorf("RoundState mismatch. Got %v; Expected %v", m2, m)
				}
			case <-ticker:
				return fmt.Errorf("Failed to read off newStepCh")
			}
		}
	case msgInfo:
		peerKey := m.PeerKey
		if peerKey == "" {
			peerKey = "local"
		}
		switch msg := m.Msg.(type) {
		case *ProposalMessage:
			p := msg.Proposal
			log.Notice("Replay: Proposal", "height", p.Height, "round", p.Round, "header",
				p.BlockPartsHeader, "pol", p.POLRound, "peer", peerKey)
		case *BlockPartMessage:
			log.Notice("Replay: BlockPart", "height", msg.Height, "round", msg.Round, "peer", peerKey)
		case *VoteMessage:
			v := msg.Vote
			log.Notice("Replay: Vote", "height", v.Height, "round", v.Round, "type", v.Type,
				"blockID", v.BlockID, "peer", peerKey)
		}

		cs.handleMsg(m, cs.RoundState)
	case timeoutInfo:
		log.Notice("Replay: Timeout", "height", m.Height, "round", m.Round, "step", m.Step, "dur", m.Duration)
		cs.handleTimeout(m, cs.RoundState)
	default:
		return fmt.Errorf("Replay: Unknown TimedWALMessage type: %v", reflect.TypeOf(msg.Msg))
	}
	return nil
}

// replay only those messages since the last block.
// timeoutRoutine should run concurrently to read off tickChan
func (cs *ConsensusState) catchupReplay(csHeight int) error {

	// set replayMode
	cs.replayMode = true
	defer func() { cs.replayMode = false }()

	// Ensure that height+1 doesn't exist
	gr, found, err := cs.wal.group.Search("#HEIGHT: ", makeHeightSearchFunc(csHeight+1))
	if found {
		return errors.New(Fmt("WAL should not contain height %d.", csHeight+1))
	}
	if gr != nil {
		gr.Close()
	}

	// Search for height marker
	gr, found, err = cs.wal.group.Search("#HEIGHT: ", makeHeightSearchFunc(csHeight))
	if err == io.EOF {
		log.Warn("Replay: wal.group.Search returned EOF", "height", csHeight)
		return nil
	} else if err != nil {
		return err
	}
	if !found {
		return errors.New(Fmt("WAL does not contain height %d.", csHeight))
	}
	defer gr.Close()

	log.Notice("Catchup by replaying consensus messages", "height", csHeight)

	for {
		line, err := gr.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return err
			}
		}
		// NOTE: since the priv key is set when the msgs are received
		// it will attempt to eg double sign but we can just ignore it
		// since the votes will be replayed and we'll get to the next step
		if err := cs.readReplayMessage([]byte(line), nil); err != nil {
			return err
		}
	}
	log.Notice("Replay: Done")
	return nil
}

//--------------------------------------------------------------------------------

// Parses marker lines of the form:
// #HEIGHT: 12345
func makeHeightSearchFunc(height int) auto.SearchFunc {
	return func(line string) (int, error) {
		line = strings.TrimRight(line, "\n")
		parts := strings.Split(line, " ")
		if len(parts) != 2 {
			return -1, errors.New("Line did not have 2 parts")
		}
		i, err := strconv.Atoi(parts[1])
		if err != nil {
			return -1, errors.New("Failed to parse INFO: " + err.Error())
		}
		if height < i {
			return 1, nil
		} else if height == i {
			return 0, nil
		} else {
			return -1, nil
		}
	}
}
