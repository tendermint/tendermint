package consensus

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	bc "github.com/tendermint/tendermint/blockchain"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
)

const (
	// event bus subscriber
	subscriber = "replay-file"
)

//--------------------------------------------------------
// replay messages interactively or all at once

func RunReplayFile(config cfg.BaseConfig, csConfig *cfg.ConsensusConfig, console bool) {
	consensusState := newConsensusStateForReplay(config, csConfig)

	if err := consensusState.ReplayFile(csConfig.WalFile(), console); err != nil {
		cmn.Exit(cmn.Fmt("Error during consensus replay: %v", err))
	}
}

// Replay msgs in file or start the console
func (cs *ConsensusState) ReplayFile(file string, console bool) error {

	if cs.IsRunning() {
		return errors.New("cs is already running, cannot replay")
	}
	if cs.wal != nil {
		return errors.New("cs wal is open, cannot replay")
	}

	cs.startForReplay()

	// ensure all new step events are regenerated as expected
	newStepCh := make(chan interface{}, 1)

	ctx := context.Background()
	err := cs.eventBus.Subscribe(ctx, subscriber, types.EventQueryNewRoundStep, newStepCh)
	if err != nil {
		return errors.Errorf("failed to subscribe %s to %v", subscriber, types.EventQueryNewRoundStep)
	}
	defer cs.eventBus.Unsubscribe(ctx, subscriber, types.EventQueryNewRoundStep)

	// just open the file for reading, no need to use wal
	fp, err := os.OpenFile(file, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}

	pb := newPlayback(file, fp, cs, cs.state.Copy())
	defer pb.fp.Close()

	var nextN int // apply N msgs in a row
	var msg *TimedWALMessage
	for {
		if nextN == 0 && console {
			nextN = pb.replayConsoleLoop()
		}

		msg, err = pb.dec.Decode()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		if err := pb.cs.readReplayMessage(msg, newStepCh); err != nil {
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

	fp    *os.File
	dec   *WALDecoder
	count int // how many lines/msgs into the file are we

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
		dec:          NewWALDecoder(fp),
	}
}

// go back count steps by resetting the state and running (pb.count - count) steps
func (pb *playback) replayReset(count int, newStepCh chan interface{}) error {
	pb.cs.Stop()
	pb.cs.Wait()

	newCS := NewConsensusState(pb.cs.config, pb.genesisState.Copy(), pb.cs.proxyAppConn, pb.cs.blockStore, pb.cs.mempool)
	newCS.SetEventBus(pb.cs.eventBus)
	newCS.startForReplay()

	pb.fp.Close()
	fp, err := os.OpenFile(pb.fileName, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}
	pb.fp = fp
	pb.dec = NewWALDecoder(fp)
	count = pb.count - count
	fmt.Printf("Reseting from %d to %d\n", pb.count, count)
	pb.count = 0
	pb.cs = newCS
	var msg *TimedWALMessage
	for i := 0; i < count; i++ {
		msg, err = pb.dec.Decode()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if err := pb.cs.readReplayMessage(msg, newStepCh); err != nil {
			return err
		}
		pb.count += 1
	}
	return nil
}

func (cs *ConsensusState) startForReplay() {
	cs.Logger.Error("Replay commands are disabled until someone updates them and writes tests")
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
			cmn.Exit("input is too long")
		} else if err != nil {
			cmn.Exit(err.Error())
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

			ctx := context.Background()
			// ensure all new step events are regenerated as expected
			newStepCh := make(chan interface{}, 1)

			err := pb.cs.eventBus.Subscribe(ctx, subscriber, types.EventQueryNewRoundStep, newStepCh)
			if err != nil {
				cmn.Exit(fmt.Sprintf("failed to subscribe %s to %v", subscriber, types.EventQueryNewRoundStep))
			}
			defer pb.cs.eventBus.Unsubscribe(ctx, subscriber, types.EventQueryNewRoundStep)

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

// convenience for replay mode
func newConsensusStateForReplay(config cfg.BaseConfig, csConfig *cfg.ConsensusConfig) *ConsensusState {
	// Get BlockStore
	blockStoreDB := dbm.NewDB("blockstore", config.DBBackend, config.DBDir())
	blockStore := bc.NewBlockStore(blockStoreDB)

	// Get State
	stateDB := dbm.NewDB("state", config.DBBackend, config.DBDir())
	state, err := sm.MakeGenesisStateFromFile(stateDB, config.GenesisFile())
	if err != nil {
		cmn.Exit(err.Error())
	}

	// Create proxyAppConn connection (consensus, mempool, query)
	clientCreator := proxy.DefaultClientCreator(config.ProxyApp, config.ABCI, config.DBDir())
	proxyApp := proxy.NewAppConns(clientCreator, NewHandshaker(state, blockStore))
	_, err = proxyApp.Start()
	if err != nil {
		cmn.Exit(cmn.Fmt("Error starting proxy app conns: %v", err))
	}

	eventBus := types.NewEventBus()
	if _, err := eventBus.Start(); err != nil {
		cmn.Exit(cmn.Fmt("Failed to start event bus: %v", err))
	}

	consensusState := NewConsensusState(csConfig, state.Copy(), proxyApp.Consensus(), blockStore, types.MockMempool{})

	consensusState.SetEventBus(eventBus)
	return consensusState
}
