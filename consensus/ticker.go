package consensus

import (
	"time"

	. "github.com/tendermint/go-common"
)

type TimeoutTicker interface {
	Start() (bool, error)
	Stop() bool
	Chan() <-chan timeoutInfo       // on which to receive a timeout
	ScheduleTimeout(ti timeoutInfo) // reset the timer
}

type timeoutTicker struct {
	BaseService

	timer    *time.Timer
	tickChan chan timeoutInfo
	tockChan chan timeoutInfo
}

func NewTimeoutTicker() TimeoutTicker {
	tt := &timeoutTicker{
		timer:    time.NewTimer(0),
		tickChan: make(chan timeoutInfo, tickTockBufferSize),
		tockChan: make(chan timeoutInfo, tickTockBufferSize),
	}
	if !tt.timer.Stop() {
		<-tt.timer.C
	}
	tt.BaseService = *NewBaseService(log, "TimeoutTicker", tt)
	return tt
}

func (t *timeoutTicker) OnStart() error {
	t.BaseService.OnStart()

	go t.timeoutRoutine()

	return nil
}

func (t *timeoutTicker) OnStop() {
	t.BaseService.OnStop()
}

func (t *timeoutTicker) Chan() <-chan timeoutInfo {
	return t.tockChan
}

// The timeoutRoutine is alwaya available to read from tickChan (it won't block).
// The scheduling may fail if the timeoutRoutine has already scheduled a timeout for a later height/round/step.
func (t *timeoutTicker) ScheduleTimeout(ti timeoutInfo) {
	t.tickChan <- ti
}

// send on tickChan to start a new timer.
// timers are interupted and replaced by new ticks from later steps
// timeouts of 0 on the tickChan will be immediately relayed to the tockChan
func (t *timeoutTicker) timeoutRoutine() {
	log.Debug("Starting timeout routine")
	var ti timeoutInfo
	for {
		select {
		case newti := <-t.tickChan:
			log.Debug("Received tick", "old_ti", ti, "new_ti", newti)

			// ignore tickers for old height/round/step
			if newti.Height < ti.Height {
				continue
			} else if newti.Height == ti.Height {
				if newti.Round < ti.Round {
					continue
				} else if newti.Round == ti.Round {
					if ti.Step > 0 && newti.Step <= ti.Step {
						continue
					}
				}
			}

			ti = newti

			// if the newti has duration == 0, we relay to the tockChan immediately (no timeout)
			if ti.Duration == time.Duration(0) {
				go func(toi timeoutInfo) { t.tockChan <- toi }(ti)
				continue
			}

			log.Debug("Scheduling timeout", "dur", ti.Duration, "height", ti.Height, "round", ti.Round, "step", ti.Step)
			if !t.timer.Stop() {
				select {
				case <-t.timer.C:
				default:
				}
			}
			t.timer.Reset(ti.Duration)
		case <-t.timer.C:
			log.Info("Timed out", "dur", ti.Duration, "height", ti.Height, "round", ti.Round, "step", ti.Step)
			// go routine here gaurantees timeoutRoutine doesn't block.
			// Determinism comes from playback in the receiveRoutine.
			// We can eliminate it by merging the timeoutRoutine into receiveRoutine
			//  and managing the timeouts ourselves with a millisecond ticker
			go func(toi timeoutInfo) { t.tockChan <- toi }(ti)
		case <-t.Quit:
			return
		}
	}
}
