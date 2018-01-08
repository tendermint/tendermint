package common

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/tendermint/tmlibs/log"
)

var (
	ErrAlreadyStarted = errors.New("already started")
	ErrAlreadyStopped = errors.New("already stopped")
)

// Service defines a service that can be started, stopped, and reset.
type Service interface {
	// Start the service.
	// If it's already started or stopped, will return an error.
	// If OnStart() returns an error, it's returned by Start()
	Start() error
	OnStart() error

	// Stop the service.
	// If it's already stopped, will return an error.
	// OnStop must never error.
	Stop() error
	OnStop()

	// Reset the service.
	// Panics by default - must be overwritten to enable reset.
	Reset() error
	OnReset() error

	// Return true if the service is running
	IsRunning() bool

	// String representation of the service
	String() string

	SetLogger(log.Logger)
}

/*
Classical-inheritance-style service declarations. Services can be started, then
stopped, then optionally restarted.

Users can override the OnStart/OnStop methods. In the absence of errors, these
methods are guaranteed to be called at most once. If OnStart returns an error,
service won't be marked as started, so the user can call Start again.

A call to Reset will panic, unless OnReset is overwritten, allowing
OnStart/OnStop to be called again.

The caller must ensure that Start and Stop are not called concurrently.

It is ok to call Stop without calling Start first.

Typical usage:

	type FooService struct {
		BaseService
		// private fields
	}

	func NewFooService() *FooService {
		fs := &FooService{
			// init
		}
		fs.BaseService = *NewBaseService(log, "FooService", fs)
		return fs
	}

	func (fs *FooService) OnStart() error {
		fs.BaseService.OnStart() // Always call the overridden method.
		// initialize private fields
		// start subroutines, etc.
	}

	func (fs *FooService) OnStop() error {
		fs.BaseService.OnStop() // Always call the overridden method.
		// close/destroy private fields
		// stop subroutines, etc.
	}
*/
type BaseService struct {
	Logger  log.Logger
	name    string
	started uint32 // atomic
	stopped uint32 // atomic
	Quit    chan struct{}

	// The "subclass" of BaseService
	impl Service
}

func NewBaseService(logger log.Logger, name string, impl Service) *BaseService {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	return &BaseService{
		Logger: logger,
		name:   name,
		Quit:   make(chan struct{}),
		impl:   impl,
	}
}

func (bs *BaseService) SetLogger(l log.Logger) {
	bs.Logger = l
}

// Implements Servce
func (bs *BaseService) Start() error {
	if atomic.CompareAndSwapUint32(&bs.started, 0, 1) {
		if atomic.LoadUint32(&bs.stopped) == 1 {
			bs.Logger.Error(Fmt("Not starting %v -- already stopped", bs.name), "impl", bs.impl)
			return ErrAlreadyStopped
		} else {
			bs.Logger.Info(Fmt("Starting %v", bs.name), "impl", bs.impl)
		}
		err := bs.impl.OnStart()
		if err != nil {
			// revert flag
			atomic.StoreUint32(&bs.started, 0)
			return err
		}
		return nil
	} else {
		bs.Logger.Debug(Fmt("Not starting %v -- already started", bs.name), "impl", bs.impl)
		return ErrAlreadyStarted
	}
}

// Implements Service
// NOTE: Do not put anything in here,
// that way users don't need to call BaseService.OnStart()
func (bs *BaseService) OnStart() error { return nil }

// Implements Service
func (bs *BaseService) Stop() error {
	if atomic.CompareAndSwapUint32(&bs.stopped, 0, 1) {
		bs.Logger.Info(Fmt("Stopping %v", bs.name), "impl", bs.impl)
		bs.impl.OnStop()
		close(bs.Quit)
		return nil
	} else {
		bs.Logger.Debug(Fmt("Stopping %v (ignoring: already stopped)", bs.name), "impl", bs.impl)
		return ErrAlreadyStopped
	}
}

// Implements Service
// NOTE: Do not put anything in here,
// that way users don't need to call BaseService.OnStop()
func (bs *BaseService) OnStop() {}

// Implements Service
func (bs *BaseService) Reset() error {
	if !atomic.CompareAndSwapUint32(&bs.stopped, 1, 0) {
		bs.Logger.Debug(Fmt("Can't reset %v. Not stopped", bs.name), "impl", bs.impl)
		return fmt.Errorf("can't reset running %s", bs.name)
	}

	// whether or not we've started, we can reset
	atomic.CompareAndSwapUint32(&bs.started, 1, 0)

	bs.Quit = make(chan struct{})
	return bs.impl.OnReset()
}

// Implements Service
func (bs *BaseService) OnReset() error {
	PanicSanity("The service cannot be reset")
	return nil
}

// Implements Service
func (bs *BaseService) IsRunning() bool {
	return atomic.LoadUint32(&bs.started) == 1 && atomic.LoadUint32(&bs.stopped) == 0
}

func (bs *BaseService) Wait() {
	<-bs.Quit
}

// Implements Servce
func (bs *BaseService) String() string {
	return bs.name
}

//----------------------------------------

type QuitService struct {
	BaseService
}

func NewQuitService(logger log.Logger, name string, impl Service) *QuitService {
	if logger != nil {
		logger.Info("QuitService is deprecated, use BaseService instead")
	}
	return &QuitService{
		BaseService: *NewBaseService(logger, name, impl),
	}
}
