package consensus

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	. "github.com/tendermint/tendermint/Godeps/_workspace/src/github.com/tendermint/go-common"
	"github.com/tendermint/tendermint/Godeps/_workspace/src/github.com/tendermint/go-wire"

	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

//--------------------------------------------------------
// types and functions for savings consensus messages

type ConsensusLogMessage struct {
	Msg ConsensusLogMessageInterface `json:"msg"`
}

type ConsensusLogMessageInterface interface{}

var _ = wire.RegisterInterface(
	struct{ ConsensusLogMessageInterface }{},
	wire.ConcreteType{&types.EventDataRoundState{}, 0x01},
	wire.ConcreteType{msgInfo{}, 0x02},
	wire.ConcreteType{timeoutInfo{}, 0x03},
)

// called in newStep and for each pass in receiveRoutine
func (cs *ConsensusState) saveMsg(msg ConsensusLogMessageInterface) {
	if cs.msgLogFP != nil {
		var n int
		var err error
		wire.WriteJSON(ConsensusLogMessage{msg}, cs.msgLogFP, &n, &err)
		wire.WriteTo([]byte("\n"), cs.msgLogFP, &n, &err) // one message per line
		if err != nil {
			log.Error("Error writing to consensus message log file", "err", err, "msg", msg)
		}
	}
}

// Open file to log all consensus messages and timeouts for deterministic accountability
func (cs *ConsensusState) OpenFileForMessageLog(file string) (err error) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.msgLogFP, err = os.OpenFile(file, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	return err
}

//--------------------------------------------------------
// replay messages

// Interactive playback
func (cs ConsensusState) ReplayConsole(file string) error {
	return cs.replay(file, true)
}

// Full playback, with tests
func (cs ConsensusState) ReplayMessages(file string) error {
	return cs.replay(file, false)
}

type playback struct {
	cs           *ConsensusState
	file         string
	fp           *os.File
	scanner      *bufio.Scanner
	newStepCh    chan interface{}
	genesisState *sm.State
	count        int
}

func newPlayback(file string, fp *os.File, cs *ConsensusState, ch chan interface{}, genState *sm.State) *playback {
	return &playback{
		cs:           cs,
		file:         file,
		newStepCh:    ch,
		genesisState: genState,
		fp:           fp,
		scanner:      bufio.NewScanner(fp),
	}
}

func (pb *playback) replayReset(count int) error {

	pb.cs.Stop()

	newCs := NewConsensusState(pb.genesisState.Copy(), pb.cs.proxyAppCtx, pb.cs.blockStore, pb.cs.mempool)
	newCs.SetEventSwitch(pb.cs.evsw)

	// we ensure all new step events are regenerated as expected
	pb.newStepCh = newCs.evsw.SubscribeToEvent("replay-test", types.EventStringNewRoundStep(), 1)

	newCs.BaseService.OnStart()
	newCs.startRoutines(0)

	pb.fp.Close()
	fp, err := os.OpenFile(pb.file, os.O_RDONLY, 0666)
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
		if err := pb.readReplayMessage(); err != nil {
			return err
		}
		pb.count += 1
	}
	return nil
}

func (cs *ConsensusState) replay(file string, console bool) error {
	if cs.IsRunning() {
		return errors.New("cs is already running, cannot replay")
	}

	cs.BaseService.OnStart()
	cs.startRoutines(0)

	if cs.msgLogFP != nil {
		cs.msgLogFP.Close()
		cs.msgLogFP = nil
	}

	// we ensure all new step events are regenerated as expected
	newStepCh := cs.evsw.SubscribeToEvent("replay-test", types.EventStringNewRoundStep(), 1)

	fp, err := os.OpenFile(file, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}

	pb := newPlayback(file, fp, cs, newStepCh, cs.state.Copy())

	defer pb.fp.Close()

	var nextN int // apply N msgs in a row
	for pb.scanner.Scan() {
		if nextN == 0 && console {
			nextN = pb.replayConsoleLoop()
		}

		if err := pb.readReplayMessage(); err != nil {
			return err
		}

		if nextN > 0 {
			nextN -= 1
		}
		pb.count += 1
	}
	return nil
}

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
			// so we restart to do this (expensive ...)

			if len(tokens) == 1 {
				pb.replayReset(1)
			} else {
				i, err := strconv.Atoi(tokens[1])
				if err != nil {
					fmt.Println("back takes an integer argument")
				} else if i > pb.count {
					fmt.Printf("argument to back must not be larger than the current count (%d)\n", pb.count)
				} else {
					pb.replayReset(i)
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
		}
	}
	return 0
}

func (pb *playback) readReplayMessage() error {
	var err error
	var msg ConsensusLogMessage
	wire.ReadJSON(&msg, pb.scanner.Bytes(), &err)
	if err != nil {
		return fmt.Errorf("Error reading json data: %v", err)
	}
	log.Notice("Replaying message", "type", reflect.TypeOf(msg.Msg), "msg", msg.Msg)
	switch m := msg.Msg.(type) {
	case *types.EventDataRoundState:
		// these are playback checks
		ticker := time.After(time.Second * 2)
		select {
		case mi := <-pb.newStepCh:
			m2 := mi.(*types.EventDataRoundState)
			if m.Height != m2.Height || m.Round != m2.Round || m.Step != m2.Step {
				return fmt.Errorf("RoundState mismatch. Got %v; Expected %v", m2, m)
			}
		case <-ticker:
			return fmt.Errorf("Failed to read off newStepCh")
		}
	case msgInfo:
		// internal or from peer
		if m.PeerKey == "" {
			pb.cs.internalMsgQueue <- m
		} else {
			pb.cs.peerMsgQueue <- m
		}
	case timeoutInfo:
		pb.cs.tockChan <- m
	default:
		return fmt.Errorf("Unknown ConsensusLogMessage type: %v", reflect.TypeOf(msg.Msg))
	}
	return nil
}
