package consensus

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"

	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// unmarshal and apply a single message to the consensus state
func (cs *ConsensusState) readReplayMessage(msgBytes []byte, newStepCh chan interface{}) error {
	var err error
	var msg ConsensusLogMessage
	wire.ReadJSON(&msg, msgBytes, &err)
	if err != nil {
		fmt.Println("MsgBytes:", msgBytes, string(msgBytes))
		return fmt.Errorf("Error reading json data: %v", err)
	}

	// for logging
	switch m := msg.Msg.(type) {
	case types.EventDataRoundState:
		log.Notice("New Step", "height", m.Height, "round", m.Round, "step", m.Step)
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
			log.Notice("Proposal", "height", p.Height, "round", p.Round, "header",
				p.BlockPartsHeader, "pol", p.POLRound, "peer", peerKey)
		case *BlockPartMessage:
			log.Notice("BlockPart", "height", msg.Height, "round", msg.Round, "peer", peerKey)
		case *VoteMessage:
			v := msg.Vote
			log.Notice("Vote", "height", v.Height, "round", v.Round, "type", v.Type,
				"hash", v.BlockHash, "header", v.BlockPartsHeader, "peer", peerKey)
		}
		// internal or from peer
		if m.PeerKey == "" {
			cs.internalMsgQueue <- m
		} else {
			cs.peerMsgQueue <- m
		}
	case timeoutInfo:
		log.Notice("Timeout", "height", m.Height, "round", m.Round, "step", m.Step, "dur", m.Duration)
		cs.tockChan <- m
	default:
		return fmt.Errorf("Unknown ConsensusLogMessage type: %v", reflect.TypeOf(msg.Msg))
	}
	return nil
}

// replay only those messages since the last block
func (cs *ConsensusState) catchupReplay(height int) error {
	if cs.wal == nil {
		log.Warn("consensus msg log is nil")
		return nil
	}
	if !cs.wal.exists {
		// new wal, nothing to catchup on
		return nil
	}

	// starting from end of file,
	// read messages until a new height is found
	nLines, err := cs.wal.SeekFromEnd(func(lineBytes []byte) bool {
		var err error
		var msg ConsensusLogMessage
		wire.ReadJSON(&msg, lineBytes, &err)
		if err != nil {
			panic(Fmt("Failed to read cs_msg_log json: %v", err))
		}
		m, ok := msg.Msg.(types.EventDataRoundState)
		if ok && m.Step == RoundStepNewHeight.String() {
			// TODO: ensure the height matches
			return true
		}
		return false
	})

	if err != nil {
		return err
	}

	var beginning bool // if we had to go back to the beginning
	if c, _ := cs.wal.fp.Seek(0, 1); c == 0 {
		beginning = true
	}

	log.Notice("Catchup by replaying consensus messages", "n", nLines)

	// now we can replay the latest nLines on consensus state
	// note we can't use scan because we've already been reading from the file
	reader := bufio.NewReader(cs.wal.fp)
	for i := 0; i < nLines; i++ {
		msgBytes, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		} else if len(msgBytes) == 0 {
			continue
		} else if len(msgBytes) == 1 && msgBytes[0] == '\n' {
			continue
		}
		// the first msg is (usually) the NewHeight event, so we can ignore it
		if !beginning && i == 1 {
			continue
		}

		// NOTE: since the priv key is set when the msgs are received
		// it will attempt to eg double sign but we can just ignore it
		// since the votes will be replayed and we'll get to the next step
		if err := cs.readReplayMessage(msgBytes, nil); err != nil {
			return err
		}
	}
	log.Info("Done catchup replay")
	return nil
}

//--------------------------------------------------------
// replay messages interactively or all at once

// Interactive playback
func (cs ConsensusState) ReplayConsole(file string) error {
	return cs.replay(file, true)
}

// Full playback, with tests
func (cs ConsensusState) ReplayMessages(file string) error {
	return cs.replay(file, false)
}

// replay all msgs or start the console
func (cs *ConsensusState) replay(file string, console bool) error {
	if cs.IsRunning() {
		return errors.New("cs is already running, cannot replay")
	}
	if cs.wal != nil {
		return errors.New("cs wal is open, cannot replay")
	}

	cs.startForReplay()

	// ensure all new step events are regenerated as expected
	newStepCh := subscribeToEvent(cs.evsw, "replay-test", types.EventStringNewRoundStep(), 1)

	// just open the file for reading, no need to use wal
	fp, err := os.OpenFile(file, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}

	pb := newPlayback(file, fp, cs, cs.state.Copy())
	defer pb.fp.Close()

	var nextN int // apply N msgs in a row
	for pb.scanner.Scan() {
		if nextN == 0 && console {
			nextN = pb.replayConsoleLoop()
		}

		if err := pb.cs.readReplayMessage(pb.scanner.Bytes(), newStepCh); err != nil {
			return err
		}

		if nextN > 0 {
			nextN -= 1
		}
		pb.count += 1
	}
	return nil
}

//------------------------------------------------
// playback manager

type playback struct {
	cs *ConsensusState

	fp      *os.File
	scanner *bufio.Scanner
	count   int // how many lines/msgs into the file are we

	// replays can be reset to beginning
	fileName     string    // so we can close/reopen the file
	genesisState *sm.State // so the replay session knows where to restart from
}

func newPlayback(fileName string, fp *os.File, cs *ConsensusState, genState *sm.State) *playback {
	return &playback{
		cs:           cs,
		fp:           fp,
		fileName:     fileName,
		genesisState: genState,
		scanner:      bufio.NewScanner(fp),
	}
}

// go back count steps by resetting the state and running (pb.count - count) steps
func (pb *playback) replayReset(count int, newStepCh chan interface{}) error {

	pb.cs.Stop()

	newCs := NewConsensusState(pb.genesisState.Copy(), pb.cs.proxyAppConn, pb.cs.blockStore, pb.cs.mempool)
	newCs.SetEventSwitch(pb.cs.evsw)

	newCs.startForReplay()

	pb.fp.Close()
	fp, err := os.OpenFile(pb.fileName, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}
	pb.fp = fp
	pb.scanner = bufio.NewScanner(fp)
	count = pb.count - count
	log.Notice(Fmt("Reseting from %d to %d", pb.count, count))
	pb.count = 0
	pb.cs = newCs
	for i := 0; pb.scanner.Scan() && i < count; i++ {
		if err := pb.cs.readReplayMessage(pb.scanner.Bytes(), newStepCh); err != nil {
			return err
		}
		pb.count += 1
	}
	return nil
}

func (cs *ConsensusState) startForReplay() {
	cs.BaseService.OnStart()
	go cs.receiveRoutine(0)
	// since we replay tocks we just ignore ticks
	go func() {
		for {
			select {
			case <-cs.tickChan:
			case <-cs.Quit:
				return
			}
		}
	}()
}

// console function for parsing input and running commands
func (pb *playback) replayConsoleLoop() int {
	for {
		fmt.Printf("> ")
		bufReader := bufio.NewReader(os.Stdin)
		line, more, err := bufReader.ReadLine()
		if more {
			Exit("input is too long")
		} else if err != nil {
			Exit(err.Error())
		}

		tokens := strings.Split(string(line), " ")
		if len(tokens) == 0 {
			continue
		}

		switch tokens[0] {
		case "next":
			// "next" -> replay next message
			// "next N" -> replay next N messages

			if len(tokens) == 1 {
				return 0
			} else {
				i, err := strconv.Atoi(tokens[1])
				if err != nil {
					fmt.Println("next takes an integer argument")
				} else {
					return i
				}
			}

		case "back":
			// "back" -> go back one message
			// "back N" -> go back N messages

			// NOTE: "back" is not supported in the state machine design,
			// so we restart and replay up to

			// ensure all new step events are regenerated as expected
			newStepCh := subscribeToEvent(pb.cs.evsw, "replay-test", types.EventStringNewRoundStep(), 1)
			if len(tokens) == 1 {
				pb.replayReset(1, newStepCh)
			} else {
				i, err := strconv.Atoi(tokens[1])
				if err != nil {
					fmt.Println("back takes an integer argument")
				} else if i > pb.count {
					fmt.Printf("argument to back must not be larger than the current count (%d)\n", pb.count)
				} else {
					pb.replayReset(i, newStepCh)
				}
			}

		case "rs":
			// "rs" -> print entire round state
			// "rs short" -> print height/round/step
			// "rs <field>" -> print another field of the round state

			rs := pb.cs.RoundState
			if len(tokens) == 1 {
				fmt.Println(rs)
			} else {
				switch tokens[1] {
				case "short":
					fmt.Printf("%v/%v/%v\n", rs.Height, rs.Round, rs.Step)
				case "validators":
					fmt.Println(rs.Validators)
				case "proposal":
					fmt.Println(rs.Proposal)
				case "proposal_block":
					fmt.Printf("%v %v\n", rs.ProposalBlockParts.StringShort(), rs.ProposalBlock.StringShort())
				case "locked_round":
					fmt.Println(rs.LockedRound)
				case "locked_block":
					fmt.Printf("%v %v\n", rs.LockedBlockParts.StringShort(), rs.LockedBlock.StringShort())
				case "votes":
					fmt.Println(rs.Votes.StringIndented("    "))

				default:
					fmt.Println("Unknown option", tokens[1])
				}
			}
		case "n":
			fmt.Println(pb.count)
		}
	}
	return 0
}
