package state

import (
	"fmt"
	"time"

	"github.com/tendermint/tendermint/libs/log"
)

var IgnoreSmbCheck bool = false

func SetIgnoreSmbCheck(check bool) {
	IgnoreSmbCheck = check
}

var lastDump int64 = time.Now().UnixNano()

type Tracer struct {
	startTime int64

	lastPin  string
	lastTime int64

	pins  []string
	times []int64
}

func (t *Tracer) pin(tag string) {
	if len(tag) == 0 {
		//panic("invalid tag")
		return
	}

	now := time.Now().UnixNano()

	if t.startTime == 0 {
		t.startTime = now
	}

	if len(t.lastPin) > 0 {
		t.pins = append(t.pins, t.lastPin)
		t.times = append(t.times, (now-t.lastTime)/1e6)
	}
	t.lastTime = now
	t.lastPin = tag
}

func (t *Tracer) dump(caller string, logger log.Logger) {
	t.pin("_")

	now := time.Now().UnixNano()
	info := fmt.Sprintf("Interval<%dms>, %s, elapsed<%dms>",
		(now-lastDump)/1e6,
		caller,
		(now-t.startTime)/1e6,
	)
	for i := range t.pins {
		info += fmt.Sprintf(", %s<%dms>", t.pins[i], t.times[i])
	}
	logger.Info(info)
	lastDump = now
}
