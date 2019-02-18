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
	cmn "github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

const (
	// event bus subscriber
	subscriber = "replay-file"
)

//--------------------------------------------------------
// replay messages interactively or all at once

// replay the wal file
func RunReplayFile(config cfg.BaseConfig, csConfig *cfg.ConsensusConfig, console bool) {
	consensusState := newConsensusStateForReplay(config, csConfig)

	if err := consensusState.ReplayFile(csConfig.WalFile(), console); err != nil {
		cmn.Exit(fmt.Sprintf("Error during consensus replay: %v", err))
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
	defer func() {
		// drain newStepCh to make sure we don't block
	LOOP:
		for {
			select {
			case <-newStepCh:
			default:
				break LOOP
			}
		}
		cs.eventBus.Unsubscribe(ctx, subscriber, types.EventQueryNewRoundStep)
	}()

	// just open the file for reading, no need to use wal
	fp, err := os.OpenFile(file, os.O_RDONLY, 0600)
	if err != nil {
		return err
	}

	pb := newPlayback(file, fp, cs, cs.state.Copy())
	defer pb.fp.Close() // nolint: errcheck

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
			nextN--
		}
		pb.count++
	}
}

//------------------------------------------------
// playback manager

type playback struct {
	cs *ConsensusState

	fp    *os.File
	dec   *WALDecoder
	count int // how many lines/msgs into the file are we

	// replays can be reset to beginning
	fileName     string   // so we can close/reopen the file
	genesisState sm.State // so the replay session knows where to restart from
}

func newPlayback(fileName string, fp *os.File, cs *ConsensusState, genState sm.State) *playback {
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

	newCS := NewConsensusState(pb.cs.config, pb.genesisState.Copy(), pb.cs.blockExec,
		pb.cs.blockStore, pb.cs.txNotifier, pb.cs.evpool)
	newCS.SetEventBus(pb.cs.eventBus)
	newCS.startForReplay()

	if err := pb.fp.Close(); err != nil {
		return err
	}
	fp, err := os.OpenFile(pb.fileName, os.O_RDONLY, 0600)
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
		pb.count++
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
			}
			i, err := strconv.Atoi(tokens[1])
			if err != nil {
				fmt.Println("next takes an integer argument")
			} else {
				return i
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
			defer func() {
				// drain newStepCh to make sure we don't block
			LOOP:
				for {
					select {
					case <-newStepCh:
					default:
						break LOOP
					}
				}
				pb.cs.eventBus.Unsubscribe(ctx, subscriber, types.EventQueryNewRoundStep)
			}()

			if len(tokens) == 1 {
				if err := pb.replayReset(1, newStepCh); err != nil {
					pb.cs.Logger.Error("Replay reset error", "err", err)
				}
			} else {
				i, err := strconv.Atoi(tokens[1])
				if err != nil {
					fmt.Println("back takes an integer argument")
				} else if i > pb.count {
					fmt.Printf("argument to back must not be larger than the current count (%d)\n", pb.count)
				} else {
					if err := pb.replayReset(i, newStepCh); err != nil {
						pb.cs.Logger.Error("Replay reset error", "err", err)
					}
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
					fmt.Println(rs.Votes.StringIndented("  "))

				default:
					fmt.Println("Unknown option", tokens[1])
				}
			}
		case "n":
			fmt.Println(pb.count)
		}
	}
}

//--------------------------------------------------------------------------------

// convenience for replay mode
func newConsensusStateForReplay(config cfg.BaseConfig, csConfig *cfg.ConsensusConfig) *ConsensusState {
	dbType := dbm.DBBackendType(config.DBBackend)
	// Get BlockStore
	blockStoreDB := dbm.NewDB("blockstore", dbType, config.DBDir())
	blockStore := bc.NewBlockStore(blockStoreDB)

	// Get State
	stateDB := dbm.NewDB("state", dbType, config.DBDir())
	gdoc, err := sm.MakeGenesisDocFromFile(config.GenesisFile())
	if err != nil {
		cmn.Exit(err.Error())
	}
	state, err := sm.MakeGenesisState(gdoc)
	if err != nil {
		cmn.Exit(err.Error())
	}

	// Create proxyAppConn connection (consensus, mempool, query)
	clientCreator := proxy.DefaultClientCreator(config.ProxyApp, config.ABCI, config.DBDir())
	proxyApp := proxy.NewAppConns(clientCreator)
	err = proxyApp.Start()
	if err != nil {
		cmn.Exit(fmt.Sprintf("Error starting proxy app conns: %v", err))
	}

	eventBus := types.NewEventBus()
	if err := eventBus.Start(); err != nil {
		cmn.Exit(fmt.Sprintf("Failed to start event bus: %v", err))
	}

	handshaker := NewHandshaker(stateDB, state, blockStore, gdoc)
	handshaker.SetEventBus(eventBus)
	err = handshaker.Handshake(proxyApp)
	if err != nil {
		cmn.Exit(fmt.Sprintf("Error on handshake: %v", err))
	}

	mempool, evpool := sm.MockMempool{}, sm.MockEvidencePool{}
	blockExec := sm.NewBlockExecutor(stateDB, log.TestingLogger(), proxyApp.Consensus(), mempool, evpool)

	consensusState := NewConsensusState(csConfig, state.Copy(), blockExec,
		blockStore, mempool, evpool)

	consensusState.SetEventBus(eventBus)
	return consensusState
}
