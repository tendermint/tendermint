package hot

import (
	"container/list"
	"math/rand"
	"sort"
	"sync"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/p2p"
)

const (
	// the type of samples, Good means get sample result in limit time, otherwise is Bad event.
	Bad eventType = iota
	Good
)

const (
	// the interval to recalculate average metrics, 10 is reasonable according to test.
	recalculateInterval = 10
	// the interval to pick decay peers
	pickDecayPeerInterval = 5000
	// only maxPermanentSetSize of best peers will stay in permanentSet
	maxPermanentSetSize = 2
	// only recent maxMetricsSampleSize of samples will be used to calculate the metrics
	maxMetricsSampleSize = 100

	// the time interval to keep consistent with peerSet in Switch
	compensateInterval = 2
)

type eventType uint

type CandidatePool struct {
	cmn.BaseService
	mtx sync.RWMutex

	pickSequence int64

	// newly added peer, will be candidates in next pick round.
	freshSet map[p2p.ID]struct{}
	// unavailable or poor performance peers, give a try periodically
	decayedSet map[p2p.ID]struct{}
	// stable and good performance peers
	permanentSet map[p2p.ID]*peerMetrics

	eventStream <-chan metricsEvent

	metrics *Metrics
	swh     *p2p.Switch
}

func NewCandidatePool(eventStream <-chan metricsEvent) *CandidatePool {
	c := &CandidatePool{
		eventStream: eventStream,
		// a random init value to avoid network resource race.
		pickSequence: rand.Int63n(pickDecayPeerInterval),
		freshSet:     make(map[p2p.ID]struct{}, 0),
		decayedSet:   make(map[p2p.ID]struct{}, 0),
		permanentSet: make(map[p2p.ID]*peerMetrics, maxPermanentSetSize),
		metrics:      NopMetrics(),
	}
	c.BaseService = *cmn.NewBaseService(nil, "CandidatePool", c)
	return c
}

// OnStart implements cmn.Service.
func (c *CandidatePool) OnStart() error {
	go c.handleSampleRoutine()
	go c.compensatePeerRoutine()

	return nil
}

func (c *CandidatePool) AddPeer(pid p2p.ID) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.freshSet[pid] = struct{}{}
}

func (c *CandidatePool) RemovePeer(pid p2p.ID) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	delete(c.freshSet, pid)
	c.tryRemoveFromDecayed(pid)
	c.tryRemoveFromPermanent(pid)
}

func (c *CandidatePool) PickCandidates() []*p2p.ID {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.pickSequence = c.pickSequence + 1
	permanentPeer := c.pickFromPermanentPeer()
	// if there is no permanentPeer, need broadcast in decayed peers.
	isBroadcast := permanentPeer == nil
	decaysPeers := c.pickFromDecayedSet(isBroadcast)
	freshPeers := c.pickFromFreshSet()
	peers := make([]*p2p.ID, 0, 1)
	if permanentPeer != nil {
		peers = append(peers, permanentPeer)
	}
	peers = append(peers, decaysPeers...)
	peers = append(peers, freshPeers...)
	return peers
}

// The `AddPeer` and `RemovePeer` function of BaseReactor have concurrent issue:
// a peer is removed and immediately added, `AddPeer` may execute before `RemovePeer`.
// Can't fix this in `stopAndRemovePeer`, because may introduce other peer dial failed.
// For `CandidatePool` it is really important to keep consistent with peerset in switch,
// this routine is an insurance.
func (c *CandidatePool) compensatePeerRoutine() {
	if c.swh == nil {
		// happened in test
		return
	}
	compensateTicker := time.NewTicker(compensateInterval)
	defer compensateTicker.Stop()
	for {
		select {
		case <-c.Quit():
			return
		case <-compensateTicker.C:
			func() {
				c.mtx.Lock()
				defer c.mtx.Unlock()
				peers := c.swh.Peers().List()
				for _, p := range peers {
					if !c.exist(p.ID()) {
						c.freshSet[p.ID()] = struct{}{}
					}
				}
				for p := range c.permanentSet {
					if !c.swh.Peers().Has(p) {
						c.tryRemoveFromPermanent(p)
					}
				}
				for p := range c.decayedSet {
					if !c.swh.Peers().Has(p) {
						c.tryRemoveFromDecayed(p)
					}
				}
				for p := range c.freshSet {
					if !c.swh.Peers().Has(p) {
						delete(c.freshSet, p)
					}
				}
			}()
		}
	}
}

func (c *CandidatePool) handleSampleRoutine() {
	for {
		select {
		case <-c.Quit():
			return
		case e := <-c.eventStream:
			c.handleMetricsEvent(e)
		}
	}
}

func (c *CandidatePool) handleMetricsEvent(e metricsEvent) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	pid := e.pid
	if !c.exist(pid) {
		c.Logger.Debug("receive a tryExpire metrics event, the peer is already removed", "peer", e.pid, "type", e.et)
		return
	}
	if e.et == Bad {
		if _, exist := c.decayedSet[pid]; !exist {
			// just try to delete it, no need to check if it exist.
			delete(c.freshSet, pid)
			c.tryRemoveFromPermanent(pid)
			c.decayedSet[pid] = struct{}{}
			c.metrics.DecayPeerSetSize.Add(1)
		} else {
			// already in decayed set, nothing need to do.
		}
	} else if e.et == Good {
		isFresh := c.isFresh(pid)
		isDecayed := c.isDecayed(pid)
		if isFresh || isDecayed {
			if isFresh {
				delete(c.freshSet, pid)
			}
			if isDecayed {
				c.tryRemoveFromDecayed(pid)
			}
			metrics := newPeerMetrics()
			metrics.addSample(e.dur)
			added, kickPeer := c.tryAddPermanentPeer(pid, metrics)
			if added {
				c.Logger.Debug("new peer joined permanent peer set", "peer", pid)
			}
			// if some peers been kicked out, just join decayed peers.
			if kickPeer != nil {
				c.decayedSet[*kickPeer] = struct{}{}
				c.metrics.DecayPeerSetSize.Add(1)
			}
		} else if metrics, exist := c.permanentSet[pid]; exist {
			metrics.addSample(e.dur)
		} else {
			// should not happen
			c.Logger.Error("receive event from peer which belongs to no peer set", "peer", e.pid)
		}
	} else {
		c.Logger.Error("receive unknown metrics event type", "type", e.et)
	}
}

func (c *CandidatePool) tryAddPermanentPeer(pid p2p.ID, metrics peerMetrics) (bool, *p2p.ID) {
	if _, exist := c.permanentSet[pid]; exist {
		c.Logger.Error("try to insert a metrics that already exists", "peer", pid)
		// unexpected things happened, the best choice is degrade this peer and try to fix.
		c.tryRemoveFromPermanent(pid)
		return false, &pid
	}
	if len(c.permanentSet) >= maxPermanentSetSize {
		//try to kick out the worst candidate.
		maxDelay := metrics.average
		kickPeer := pid
		for p, m := range c.permanentSet {
			if maxDelay < m.average {
				maxDelay = m.average
				kickPeer = p
			}
		}
		if kickPeer != pid {
			c.tryRemoveFromPermanent(kickPeer)
			c.permanentSet[pid] = &metrics
			c.metrics.PermanentPeerSetSize.Add(1)
			c.metrics.PermanentPeers.With("peer_id", string(pid)).Set(1)
			return true, &kickPeer
		}
		return false, &pid
	} else {
		c.permanentSet[pid] = &metrics
		c.metrics.PermanentPeerSetSize.Add(1)
		c.metrics.PermanentPeers.With("peer_id", string(pid)).Set(1)
		return true, nil
	}
}

func (c *CandidatePool) pickFromFreshSet() []*p2p.ID {
	peers := make([]*p2p.ID, 0, len(c.freshSet))
	for peer := range c.freshSet {
		peers = append(peers, &peer)
	}
	return peers
}

func (c *CandidatePool) pickFromDecayedSet(broadcast bool) []*p2p.ID {
	if len(c.decayedSet) == 0 {
		return []*p2p.ID{}
	}
	peers := make([]*p2p.ID, 0, len(c.decayedSet))
	for peer := range c.decayedSet {
		peers = append(peers, &peer)
	}
	if !broadcast {
		if c.pickSequence%pickDecayPeerInterval == 0 {
			index := rand.Intn(len(peers))
			return []*p2p.ID{peers[index]}
		}
		return []*p2p.ID{}
	} else {
		return peers
	}
}

// larger average while less opportunity to be chosen. example:
// peer         :          peerA  peerB  peerC
// average delay:          1ns    1ns    8ns
// percentage   :          0.1    0.1    0.8
// diceSection  :          [0.9,  1.8,   2.0]
// choose opportunity:     45%,   45%,   10%
func (c *CandidatePool) pickFromPermanentPeer() *p2p.ID {
	size := len(c.permanentSet)
	if size == 0 {
		return nil
	}
	diceSection := make([]float64, 0, size)
	peers := make([]p2p.ID, 0, size)
	var total, section float64
	for _, m := range c.permanentSet {
		total += float64(m.average)
	}
	// total could not be 0 actually, but assign with 1 if it unfortunately happened
	if total == 0 {
		total = 1
	}
	for p, m := range c.permanentSet {
		peers = append(peers, p)
		section = section + (1 - float64(m.average)/total)
		diceSection = append(diceSection, section)
	}
	diceValue := rand.Float64() * diceSection[len(diceSection)-1]
	choose := sort.SearchFloat64s(diceSection, diceValue)
	return &peers[choose]
}

func (c *CandidatePool) exist(pid p2p.ID) bool {
	if c.isPermanent(pid) {
		return true
	} else if c.isDecayed(pid) {
		return true
	} else if c.isFresh(pid) {
		return true
	}
	return false
}

func (c *CandidatePool) isFresh(pid p2p.ID) bool {
	_, exist := c.freshSet[pid]
	return exist
}

func (c *CandidatePool) isDecayed(pid p2p.ID) bool {
	_, exist := c.decayedSet[pid]
	return exist
}

func (c *CandidatePool) isPermanent(pid p2p.ID) bool {
	_, exist := c.permanentSet[pid]
	return exist
}

func (c *CandidatePool) tryRemoveFromPermanent(pid p2p.ID) {
	if c.isPermanent(pid) {
		delete(c.permanentSet, pid)
		c.metrics.PermanentPeerSetSize.Set(-1)
		c.metrics.PermanentPeers.With("peer_id", string(pid)).Set(0)
	}
}

func (c *CandidatePool) tryRemoveFromDecayed(pid p2p.ID) {
	if c.isDecayed(pid) {
		delete(c.decayedSet, pid)
		c.metrics.DecayPeerSetSize.Set(-1)
	}
}

// --------------------------------------------------
type metricsEvent struct {
	et  eventType
	pid p2p.ID
	dur int64 // nano second
}

type peerMetrics struct {
	sampleSequence int64

	samples *list.List
	average int64 // nano second
}

func newPeerMetrics() peerMetrics {
	return peerMetrics{
		samples:        list.New(),
		sampleSequence: rand.Int63n(recalculateInterval),
	}
}

func (metrics *peerMetrics) addSample(dur int64) {
	metrics.sampleSequence = metrics.sampleSequence + 1
	// fast calculate average, but error accumulates
	if metrics.samples.Len() >= maxMetricsSampleSize {
		// shift old sample
		popItem := metrics.samples.Front()
		metrics.samples.Remove(popItem)
		popDur := *popItem.Value.(*int64)
		// no worry about int64 overflow, the max dur will not exceed math.MaxInt64/maxMetricsSampleSize
		metrics.average = (metrics.average*int64(maxMetricsSampleSize) + (dur - popDur)) / int64(maxMetricsSampleSize)
	} else {
		length := int64(metrics.samples.Len())
		metrics.average = (metrics.average*length + dur) / (length + 1)
	}
	metrics.samples.PushBack(&dur)

	// correction error periodically
	if metrics.sampleSequence%recalculateInterval == 0 {
		var totalDur int64
		// no worry about int64 overflow, the max dur will not exceed math.MaxInt64/maxMetricsSampleSize
		s := metrics.samples.Front()
		for s != nil {
			sdur := *s.Value.(*int64)
			totalDur += sdur
			s = s.Next()
		}
		metrics.average = totalDur / int64(metrics.samples.Len())
	}
}
