package common

import "sync/atomic"

// BaseService represents a service that can be started then stopped,
// but cannot be restarted.
// .Start() calls the onStart callback function, and .Stop() calls onStop.
// It is meant to be embedded into service structs.
// The user must ensure that Start() and Stop() are not called concurrently.
// It is ok to call Stop() without calling Start() first -- the onStop
// callback will be called, and the service will never start.
type BaseService struct {
	name    string
	service interface{} // for log statements.
	started uint32      // atomic
	stopped uint32      // atomic
	onStart func()
	onStop  func()
}

func NewBaseService(name string, service interface{}, onStart, onStop func()) *BaseService {
	return &BaseService{
		name:    name,
		service: service,
		onStart: onStart,
		onStop:  onStop,
	}
}

func (bs *BaseService) Start() bool {
	if atomic.CompareAndSwapUint32(&bs.started, 0, 1) {
		if atomic.LoadUint32(&bs.stopped) == 1 {
			log.Warn(Fmt("Not starting %v -- already stopped", bs.name), "service", bs.service)
			return false
		} else {
			log.Notice(Fmt("Starting %v", bs.name), "service", bs.service)
		}
		if bs.onStart != nil {
			bs.onStart()
		}
		return true
	} else {
		log.Info(Fmt("Not starting %v -- already started", bs.name), "service", bs.service)
		return false
	}
}

func (bs *BaseService) Stop() bool {
	if atomic.CompareAndSwapUint32(&bs.stopped, 0, 1) {
		log.Notice(Fmt("Stopping %v", bs.name), "service", bs.service)
		if bs.onStop != nil {
			bs.onStop()
		}
		return true
	} else {
		log.Notice(Fmt("Not stopping %v", bs.name), "service", bs.service)
		return false
	}
}

func (bs *BaseService) IsRunning() bool {
	return atomic.LoadUint32(&bs.started) == 1 && atomic.LoadUint32(&bs.stopped) == 0
}
