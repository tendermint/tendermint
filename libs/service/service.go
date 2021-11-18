package service

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/tendermint/tendermint/libs/log"
)

var (
	// ErrAlreadyStarted is returned when somebody tries to start an already
	// running service.
	ErrAlreadyStarted = errors.New("already started")
	// ErrAlreadyStopped is returned when somebody tries to stop an already
	// stopped service (without resetting it).
	ErrAlreadyStopped = errors.New("already stopped")
	// ErrNotStarted is returned when somebody tries to stop a not running
	// service.
	ErrNotStarted = errors.New("not started")
)

// Service defines a service that can be started, stopped, and reset.
type Service interface {
	// Start is called to start the service, which should run until
	// the context terminates. If the service is already running, Start
	// must report an error.
	Start(context.Context) error

	// Return true if the service is running
	IsRunning() bool

	// Quit returns a channel, which is closed once service is stopped.
	Quit() <-chan struct{}

	// String representation of the service
	String() string

	// Wait blocks until the service is stopped.
	Wait()
}

// Implementation describes the implementation that the
// BaseService implementation wraps.
type Implementation interface {
	Service

	// Called by the Services Start Method
	OnStart(context.Context) error

	// Called when the service's context is canceled.
	OnStop()
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

	func (fs *FooService) OnStart(ctx context.Context) error {
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
	quit    chan struct{}

	// The "subclass" of BaseService
	impl Implementation
}

// NewBaseService creates a new BaseService.
func NewBaseService(logger log.Logger, name string, impl Implementation) *BaseService {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	return &BaseService{
		Logger: logger,
		name:   name,
		quit:   make(chan struct{}),
		impl:   impl,
	}
}

// Start starts the Service and calls its OnStart method. An error will be
// returned if the service is already running or stopped.  To restart a
// stopped service, call Reset.
func (bs *BaseService) Start(ctx context.Context) error {
	if atomic.CompareAndSwapUint32(&bs.started, 0, 1) {
		if atomic.LoadUint32(&bs.stopped) == 1 {
			bs.Logger.Error("not starting service; already stopped", "service", bs.name, "impl", bs.impl.String())
			atomic.StoreUint32(&bs.started, 0)
			return ErrAlreadyStopped
		}

		bs.Logger.Info("starting service", "service", bs.name, "impl", bs.impl.String())

		if err := bs.impl.OnStart(ctx); err != nil {
			// revert flag
			atomic.StoreUint32(&bs.started, 0)
			return err
		}

		go func(ctx context.Context) {
			<-ctx.Done()
			if err := bs.Stop(); err != nil {
				bs.Logger.Error("stopped service",
					"err", err.Error(),
					"service", bs.name,
					"impl", bs.impl.String())
			}

			bs.Logger.Info("stopped service",
				"service", bs.name,
				"impl", bs.impl.String())
		}(ctx)

		return nil
	}

	bs.Logger.Debug("not starting service; already started", "service", bs.name, "impl", bs.impl.String())
	return ErrAlreadyStarted
}

// OnStart implements Service by doing nothing.
// NOTE: Do not put anything in here,
// that way users don't need to call BaseService.OnStart()
func (bs *BaseService) OnStart(ctx context.Context) error { return nil }

// Stop implements Service by calling OnStop (if defined) and closing quit
// channel. An error will be returned if the service is already stopped.
func (bs *BaseService) Stop() error {
	if atomic.CompareAndSwapUint32(&bs.stopped, 0, 1) {
		if atomic.LoadUint32(&bs.started) == 0 {
			bs.Logger.Error("not stopping service; not started yet", "service", bs.name, "impl", bs.impl.String())
			atomic.StoreUint32(&bs.stopped, 0)
			return ErrNotStarted
		}

		bs.Logger.Info("stopping service", "service", bs.name, "impl", bs.impl.String())
		bs.impl.OnStop()
		close(bs.quit)

		return nil
	}

	bs.Logger.Debug("not stopping service; already stopped", "service", bs.name, "impl", bs.impl.String())
	return ErrAlreadyStopped
}

// OnStop implements Service by doing nothing.
// NOTE: Do not put anything in here,
// that way users don't need to call BaseService.OnStop()
func (bs *BaseService) OnStop() {}

// IsRunning implements Service by returning true or false depending on the
// service's state.
func (bs *BaseService) IsRunning() bool {
	return atomic.LoadUint32(&bs.started) == 1 && atomic.LoadUint32(&bs.stopped) == 0
}

// Wait blocks until the service is stopped.
func (bs *BaseService) Wait() { <-bs.quit }

// String implements Service by returning a string representation of the service.
func (bs *BaseService) String() string { return bs.name }

// Quit Implements Service by returning a quit channel.
func (bs *BaseService) Quit() <-chan struct{} { return bs.quit }
