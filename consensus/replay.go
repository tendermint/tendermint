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

	auto "github.com/tendermint/go-autofile"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"

	sm "github.com/tendermint/tendermint/state"
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
	pb.cs.Wait()

	newCS := NewConsensusState(pb.cs.config, pb.genesisState.Copy(), pb.cs.proxyAppConn, pb.cs.blockStore, pb.cs.mempool)
	newCS.SetEventSwitch(pb.cs.evsw)
	newCS.startForReplay()

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
	pb.cs = newCS
	for i := 0; pb.scanner.Scan() && i < count; i++ {
		if err := pb.cs.readReplayMessage(pb.scanner.Bytes(), newStepCh); err != nil {
			return err
		}
		pb.count += 1
	}
	return nil
}

func (cs *ConsensusState) startForReplay() {
	// don't want to start full cs
	cs.BaseService.OnStart()

	log.Warn("Replay commands are disabled until someone updates them and writes tests")
	/* TODO:!
	// since we replay tocks we just ignore ticks
		go func() {
			for {
				select {
				case <-cs.tickChan:
				case <-cs.Quit:
					return
				}
			}
		}()*/
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
