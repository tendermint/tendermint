package service

import (
	"context"
	"errors"
	"sync"

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
	logger log.Logger
	name   string
	mtx    sync.Mutex
	quit   <-chan (struct{})
	cancel context.CancelFunc

	// The "subclass" of BaseService
	impl Implementation
}

// NewBaseService creates a new BaseService.
func NewBaseService(logger log.Logger, name string, impl Implementation) *BaseService {
	return &BaseService{
		logger: logger,
		name:   name,
		impl:   impl,
	}
}

// Start starts the Service and calls its OnStart method. An error will be
// returned if the service is already running or stopped.  To restart a
// stopped service, call Reset.
func (bs *BaseService) Start(ctx context.Context) error {
	bs.mtx.Lock()
	defer bs.mtx.Unlock()

	if bs.quit != nil {
		return ErrAlreadyStarted
	}

	select {
	case <-bs.quit:
		bs.logger.Error("not starting service; already stopped", "service", bs.name, "impl", bs.impl.String())
		return ErrAlreadyStopped
	default:
		bs.logger.Info("starting service", "service", bs.name, "impl", bs.impl.String())
		if err := bs.impl.OnStart(ctx); err != nil {
			return err
		}

		// we need a separate context to ensure that we start
		// a thread that will get cleaned up and that the
		// Stop/Wait functions work as expected.
		srvCtx, cancel := context.WithCancel(context.Background())
		bs.cancel = cancel
		bs.quit = srvCtx.Done()

		go func(ctx context.Context) {
			select {
			case <-srvCtx.Done():
				// this means stop was called manually
				return
			case <-ctx.Done():
				_ = bs.Stop()
			}

			bs.logger.Info("stopped service",
				"service", bs.name,
				"impl", bs.impl.String())
		}(ctx)

		return nil
	}
}

// Stop implements Service by calling OnStop (if defined) and closing quit
// channel. An error will be returned if the service is already stopped.
func (bs *BaseService) Stop() error {
	bs.mtx.Lock()
	defer bs.mtx.Unlock()

	if bs.quit == nil {
		bs.logger.Error("not stopping service; not started yet", "service", bs.name, "impl", bs.impl.String())
		return ErrNotStarted
	}

	select {
	case <-bs.quit:
		return ErrAlreadyStopped
	default:
		bs.logger.Info("stopping service", "service", bs.name, "impl", bs.impl.String())
		bs.impl.OnStop()
		bs.cancel()

		return nil
	}
}

// IsRunning implements Service by returning true or false depending on the
// service's state.
func (bs *BaseService) IsRunning() bool {
	bs.mtx.Lock()
	defer bs.mtx.Unlock()

	if bs.quit == nil {
		return false
	}

	select {
	case <-bs.quit:
		return false
	default:
		return true
	}
}

// Wait blocks until the service is stopped.
func (bs *BaseService) Wait() { <-bs.quit }

// String implements Service by returning a string representation of the service.
func (bs *BaseService) String() string { return bs.name }
