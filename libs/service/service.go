package service

import (
	"context"
	"errors"
	"sync"

	"github.com/tendermint/tendermint/libs/log"
)

var (
	// errAlreadyStopped is returned when somebody tries to stop an already
	// stopped service (without resetting it).
	errAlreadyStopped = errors.New("already stopped")

	_ Service = (*BaseService)(nil)
)

// Service defines a service that can be started, stopped, and reset.
type Service interface {
	// Start is called to start the service, which should run until
	// the context terminates. If the service is already running, Start
	// must report an error.
	Start(context.Context) error

	// Return true if the service is running
	IsRunning() bool

	// Wait blocks until the service is stopped.
	Wait()
}

// Implementation describes the implementation that the
// BaseService implementation wraps.
type Implementation interface {
	// Called by the Services Start Method
	OnStart(context.Context) error

	// Called when the service's context is canceled.
	OnStop()
}

/*
Classical-inheritance-style service declarations. Services can be started, then
stopped, but cannot be restarted.

Users must implement OnStart/OnStop methods. In the absence of errors, these
methods are guaranteed to be called at most once. If OnStart returns an error,
service won't be marked as started, so the user can call Start again.

The BaseService implementation ensures that the OnStop method is
called after the context passed to Start is canceled.

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
		// initialize private fields
		// start subroutines, etc.
	}

	func (fs *FooService) OnStop() {
		// close/destroy private fields and releases resources
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

// Start starts the Service and calls its OnStart method. An error
// will be returned if the service is stopped, but not if it is
// already running.
func (bs *BaseService) Start(ctx context.Context) error {
	bs.mtx.Lock()
	defer bs.mtx.Unlock()

	if bs.quit != nil {
		return nil
	}

	select {
	case <-bs.quit:
		return errAlreadyStopped
	default:
		bs.logger.Info("starting service", "service", bs.name, "impl", bs.name)
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
				bs.Stop()
			}

			bs.logger.Info("stopped service",
				"service", bs.name)
		}(ctx)

		return nil
	}
}

// Stop manually terminates the service by calling OnStop method from
// the implementation and releases all resources related to the
// service.
func (bs *BaseService) Stop() {
	bs.mtx.Lock()
	defer bs.mtx.Unlock()

	if bs.quit == nil {
		return
	}

	select {
	case <-bs.quit:
		return
	default:
		bs.logger.Info("stopping service", "service", bs.name)
		bs.impl.OnStop()
		bs.cancel()

		return
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

func (bs *BaseService) getWait() <-chan struct{} {
	bs.mtx.Lock()
	defer bs.mtx.Unlock()

	if bs.quit == nil {
		out := make(chan struct{})
		close(out)
		return out
	}

	return bs.quit
}

// Wait blocks until the service is stopped.
func (bs *BaseService) Wait() { <-bs.getWait() }

// String provides a human-friendly representation of the service.
func (bs *BaseService) String() string { return bs.name }
