package consensus

import (
	"fmt"
	"time"

	"github.com/tendermint/tendermint/types"

	cstypes "github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/libs/log"
)

type coreData struct {
	newRoundStepTime    int64
	newRoundStepTimeEnd int64
	proposeStepTime     int64
	proposeStepTimeEnd  int64
	prevoteStepTime     int64
	prevoteStepWaitTime int64
	prevoteStepTimeEnd  int64
	precommitTime       int64
	precommitWaitTime   int64
	precommitTimeEnd    int64
	commitTime          int64
	commitTimeEnd       int64
	prevoteSteps        int
	prevoteAddress      []string
	precommitSteps      int
	precommitAddress    []string
	proposeNode         string
	isProposer          bool
}

type consensusTrack struct {
	coreTrack   map[int64]*coreData
	trackSwitch bool
	l           log.Logger
}

func newConsensusTrack(trackSwitch bool) *consensusTrack {
	return &consensusTrack{make(map[int64]*coreData), trackSwitch, nil}
}

func (c *consensusTrack) increaseCount(height int64, opt types.SignedMsgType, addr string) {
	core := c.coreTrack[height]
	if core != nil {
		switch opt {
		case types.PrevoteType:
			core.prevoteSteps++
			core.prevoteAddress = append(core.prevoteAddress, addr)
		case types.PrecommitType:
			core.precommitSteps++
			core.precommitAddress = append(core.precommitAddress, addr)
		}
	}
}

func (c *consensusTrack) setBlockProposer(height int64, peer string) {
	core := c.coreTrack[height]
	if core != nil {
		core.proposeNode = peer
	}
}

func (c *consensusTrack) setIsProposer(height int64, bTurn bool) {
	core := c.coreTrack[height]
	if core != nil {
		core.isProposer = bTurn
	}
}

func (c *consensusTrack) setTrace(height int64, r cstypes.RoundStepType, begin bool) {

	if !c.trackSwitch {
		return
	}
	core := c.coreTrack[height]
	if core == nil {
		core = &coreData{}
		c.coreTrack[height] = core
	}
	switch r {
	case cstypes.RoundStepNewRound:
		if begin {
			core.newRoundStepTime = time.Now().UnixNano()
		} else {
			core.newRoundStepTimeEnd = time.Now().UnixNano()
		}
	case cstypes.RoundStepPropose:
		if begin {
			core.proposeStepTime = time.Now().UnixNano()
		} else {
			core.proposeStepTimeEnd = time.Now().UnixNano()
		}
	case cstypes.RoundStepPrevote:
		if begin {
			core.prevoteStepTime = time.Now().UnixNano()
		} else {
			core.prevoteStepTimeEnd = time.Now().UnixNano()
		}
	case cstypes.RoundStepPrecommit:
		if begin {
			core.precommitTime = time.Now().UnixNano()
		} else {
			core.precommitTimeEnd = time.Now().UnixNano()
		}
	case cstypes.RoundStepCommit:
		if begin {
			core.commitTime = time.Now().UnixNano()
		} else {
			core.commitTimeEnd = time.Now().UnixNano()
		}
	case cstypes.RoundStepPrevoteWait:
		core.prevoteStepWaitTime = time.Now().UnixNano()
	case cstypes.RoundStepPrecommitWait:
		core.precommitWaitTime = time.Now().UnixNano()
	}
}

func (c *consensusTrack) calcPeriod(startTime int64, endTime int64) int64 {
	if startTime == 0 || endTime == 0 {
		return 0
	}
	return endTime - startTime
}

func (c *consensusTrack) display(height int64) {
	if !c.trackSwitch || c.l == nil {
		return
	}

	core := c.coreTrack[height]
	if core == nil {
		return
	}

	total := c.calcPeriod(core.newRoundStepTime, core.newRoundStepTimeEnd) +
		c.calcPeriod(core.proposeStepTime, core.proposeStepTimeEnd) +
		c.calcPeriod(core.prevoteStepTime, core.prevoteStepTimeEnd) +
		c.calcPeriod(core.precommitTime, core.precommitTimeEnd) +
		c.calcPeriod(core.commitTime, core.commitTimeEnd)

	ourTurn := "This block was proposed by current local node"
	if !core.isProposer {
		ourTurn = "This block wasn't proposed by current local node"
	}

	logFunc := c.l.Debug
	if total > 10000 { //if total time is more than 10s, Print log info by Info
		logFunc = c.l.Info
	}

	logFunc("==========================consensus track==========================")
	logFunc(fmt.Sprintf("Height：[%v] - RoundStepNewRound<%vms>,S:%vms,E:%vms",
		height, c.calcPeriod(core.newRoundStepTime, core.newRoundStepTimeEnd),
		core.newRoundStepTime, core.newRoundStepTimeEnd))
	logFunc(fmt.Sprintf("Height：[%v] - RoundStepPropose<%vms>,S:%vms,E:%vms",
		height, c.calcPeriod(core.proposeStepTime, core.proposeStepTimeEnd),
		core.proposeStepTime, core.proposeStepTimeEnd))
	logFunc(fmt.Sprintf("Height：[%v] - RoundStepPrevote<%vms>,S:%vms,E:%vms",
		height, c.calcPeriod(core.prevoteStepTime, core.prevoteStepTimeEnd),
		core.prevoteStepTime, core.prevoteStepTimeEnd))
	logFunc(fmt.Sprintf("Height：[%v] - RoundStepPrevoteWait<%vms>,S:%vms,E:%vms",
		height, c.calcPeriod(core.prevoteStepWaitTime, core.prevoteStepTimeEnd),
		core.prevoteStepWaitTime, core.prevoteStepTimeEnd))
	logFunc(fmt.Sprintf("Height：[%v] - RoundStepPrecommit<%vms>,S:%vms,E:%vms",
		height, c.calcPeriod(core.precommitTime, core.precommitTimeEnd),
		core.precommitTime, core.precommitTimeEnd))
	logFunc(fmt.Sprintf("Height：[%v] - RoundStepPrecommitWait<%vms>,S:%vms,E:%vms",
		height, c.calcPeriod(core.precommitWaitTime, core.precommitTimeEnd),
		core.precommitWaitTime, core.precommitTimeEnd))
	logFunc(fmt.Sprintf("Height：[%v] - RoundStepCommit<%vms>,S:%vms,E:%vms",
		height, c.calcPeriod(core.commitTime, core.commitTimeEnd),
		core.commitTime, core.commitTimeEnd))
	logFunc(fmt.Sprintf("Height：[%v] - Total Time [%v ms]", height, total))
	logFunc(fmt.Sprintf("Height：[%v] - block prevote <%v> times, precommit <%v> times, propose peer: <%v>,%v",
		height, core.prevoteSteps, core.precommitSteps, core.proposeNode, ourTurn))

	for _, addr := range core.prevoteAddress {
		logFunc(fmt.Sprintf("Height：[%v] - Prevote peer:[%v]", height, addr))
	}

	for _, addr := range core.precommitAddress {
		logFunc(fmt.Sprintf("Height：[%v] - Precommit peer:[%v]", height, addr))
	}

	if total > 20000 {
		logFunc(fmt.Sprintf("Height：[%v] - Timeout Alert! consensus more than 20s", height))
	}
	logFunc("==============================================end consensus track=======================================================")

	//free displayed data
	if height > 0 && len(c.coreTrack) > 2 {
		delete(c.coreTrack, height-1)
	}
}

var track = newConsensusTrack(true)

//------------end of
func (cs *State) calcProcessingTime(height int64, stepType cstypes.RoundStepType) {
	core := track.coreTrack[height]
	if core == nil {
		core = &coreData{}
		track.coreTrack[height] = core
	}
	switch stepType {
	case cstypes.RoundStepNewRound:
		cs.metrics.NewRoundProcessingTime.Set(float64(track.calcPeriod(core.newRoundStepTime, core.newRoundStepTimeEnd)) / 1e6)
	case cstypes.RoundStepPropose:
		cs.metrics.ProposeProcessingTime.Set(float64(track.calcPeriod(core.proposeStepTime, core.proposeStepTimeEnd)) / 1e6)
	case cstypes.RoundStepPrevote:
		cs.metrics.PrevoteProcessingTime.Set(float64(track.calcPeriod(core.prevoteStepTime, core.prevoteStepTimeEnd)) / 1e6)
	case cstypes.RoundStepPrecommit:
		cs.metrics.PrecommitProcessingTime.Set(float64(track.calcPeriod(core.precommitTime, core.precommitTimeEnd)) / 1e6)
	case cstypes.RoundStepCommit:
		cs.metrics.CommitProcessingTime.Set(float64(track.calcPeriod(core.commitTime, core.commitTimeEnd)) / 1e6)
	default:
		break
	}
}
