package consensus

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	dbm "github.com/tendermint/go-db"
	bc "github.com/tendermint/tendermint/blockchain"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

//--------------------------------------------------------
// replay messages interactively or all at once

func RunReplayFile(config cfg.Config, walFile string, console bool) {
	consensusState := newConsensusStateForReplay(config)

	if err := consensusState.ReplayFile(walFile, console); err != nil {
		Exit(Fmt("Error during consensus replay: %v", err))
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

// convenience for replay mode
func newConsensusStateForReplay(config cfg.Config) *ConsensusState {
	// Get BlockStore
	blockStoreDB := dbm.NewDB("blockstore", config.GetString("db_backend"), config.GetString("db_dir"))
	blockStore := bc.NewBlockStore(blockStoreDB)

	// Get State
	stateDB := dbm.NewDB("state", config.GetString("db_backend"), config.GetString("db_dir"))
	state := sm.MakeGenesisStateFromFile(stateDB, config.GetString("genesis_file"))

	// Create proxyAppConn connection (consensus, mempool, query)
	proxyApp := proxy.NewAppConns(config, proxy.DefaultClientCreator(config), NewHandshaker(config, state, blockStore))
	_, err := proxyApp.Start()
	if err != nil {
		Exit(Fmt("Error starting proxy app conns: %v", err))
	}

	// add the chainid to the global config
	config.Set("chain_id", state.ChainID)

	// Make event switch
	eventSwitch := types.NewEventSwitch()
	if _, err := eventSwitch.Start(); err != nil {
		Exit(Fmt("Failed to start event switch: %v", err))
	}

	mempool := mempl.NewMempool(config, proxyApp.Mempool())

	consensusState := NewConsensusState(config, state.Copy(), proxyApp.Consensus(), blockStore, mempool)
	consensusState.SetEventSwitch(eventSwitch)
	return consensusState
}
