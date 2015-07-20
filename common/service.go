/*

Classical-inheritance-style service declarations.
Services can be started, then stopped.
Users can override the AfterStart/AfterStop methods.
These methods are guaranteed to be called at most once.
Caller must ensure that Start() and Stop() are not called concurrently.
It is ok to call Stop() without calling Start() first.
Services cannot be re-started unless otherwise documented.

Typical usage:

type FooService struct {
	BaseService
	// private fields
}

func NewFooService() *FooService {
	fs := &FooService{
		// init
	}
	fs.BaseService = *BaseService(log, "FooService", fs)
	return fs
}

func (fs *FooService) AfterStart() {
	// initialize private fields
	// start subroutines, etc.
}

func (fs *FooService) AfterStart() {
	// close/destroy private fields
	// stop subroutines, etc.
}

*/
package common

import "sync/atomic"
import "github.com/tendermint/tendermint/Godeps/_workspace/src/github.com/tendermint/log15"

type Service interface {
	Start() bool
	BeforeStart()
	AfterStart()

	Stop() bool
	BeforeStop()
	AfterStop()

	IsRunning() bool

	String() string
}

type BaseService struct {
	log     log15.Logger
	name    string
	started uint32 // atomic
	stopped uint32 // atomic

	// The "subclass" of BaseService
	impl Service
}

func NewBaseService(log log15.Logger, name string, impl Service) *BaseService {
	return &BaseService{
		log:  log,
		name: name,
		impl: impl,
	}
}

// Implements Servce
func (bs *BaseService) Start() bool {
	if atomic.CompareAndSwapUint32(&bs.started, 0, 1) {
		if atomic.LoadUint32(&bs.stopped) == 1 {
			bs.log.Warn(Fmt("Not starting %v -- already stopped", bs.name), "impl", bs.impl)
			return false
		} else {
			bs.log.Notice(Fmt("Starting %v", bs.name), "impl", bs.impl)
		}
		bs.impl.BeforeStart()
		bs.impl.AfterStart()
		return true
	} else {
		bs.log.Info(Fmt("Not starting %v -- already started", bs.name), "impl", bs.impl)
		return false
	}
}

// Implements Service
func (bs *BaseService) BeforeStart() {}

// Implements Service
func (bs *BaseService) AfterStart() {}

// Implements Service
func (bs *BaseService) Stop() bool {
	if atomic.CompareAndSwapUint32(&bs.stopped, 0, 1) {
		bs.log.Notice(Fmt("Stopping %v", bs.name), "impl", bs.impl)
		bs.impl.BeforeStop()
		bs.impl.AfterStop()
		return true
	} else {
		bs.log.Notice(Fmt("Not stopping %v", bs.name), "impl", bs.impl)
		return false
	}
}

// Implements Service
func (bs *BaseService) BeforeStop() {}

// Implements Service
func (bs *BaseService) AfterStop() {}

// Implements Service
func (bs *BaseService) IsRunning() bool {
	return atomic.LoadUint32(&bs.started) == 1 && atomic.LoadUint32(&bs.stopped) == 0
}

// Implements Servce
func (bs *BaseService) String() string {
	return bs.name
}

//----------------------------------------

type QuitService struct {
	BaseService
	Quit chan struct{}
}

func NewQuitService(log log15.Logger, name string, impl Service) *QuitService {
	return &QuitService{
		BaseService: *NewBaseService(log, name, impl),
		Quit:        nil,
	}
}

// Init .Quit in BeforeStart such that AfterStart of impls have access to Quit.
// NOTE: When overriding BeforeStart, call QuitService.BeforeStart() manually.
func (qs *QuitService) BeforeStart() {
	qs.Quit = make(chan struct{})
}

// Close .Quit after Stop/BeforeStop/AfterStop
func (qs *QuitService) Stop() bool {
	res := qs.BaseService.Stop()
	if res {
		close(qs.Quit)
	}
	return res
}
