package service

import (
	"fmt"

	"github.com/tendermint/tendermint/libs/log"
)

type groupImpl struct {
	*BaseService
	services []Service
}

func NewGroup(logger log.Logger, name string, services ...Service) Service {
	srv := &groupImpl{
		services: services,
	}
	srv.BaseService = NewBaseService(logger, name, srv)
	return srv
}

func (gs *groupImpl) OnStart() error {
	for _, srv := range gs.services {
		if err := srv.Reset(); err != nil {
			return err
		}
	}
	return nil
}

func (gs *groupImpl) OnStop() {
	for idx, srv := range gs.services {
		if err := srv.Stop(); err != nil {
			gs.Logger.Error(
				fmt.Sprintf("problem starting service %d of %d", idx, len(gs.services)),
				"err", err)
		}
	}
}

func (gs *groupImpl) OnReset() error {
	for _, srv := range gs.services {
		if err := srv.Reset(); err != nil {
			return err
		}
	}
	return nil
}

type FunctionalService struct {
	Starter func() error
	Stopper func()
	Reseter func() error
}

func MakeFunctionalService(logger log.Logger, name string, opts FunctionalService) Service {
	srv := &funImpl{
		ops: opts,
	}

	srv.BaseService = NewBaseService(logger, name, srv)
	return srv
}

type funImpl struct {
	*BaseService
	ops FunctionalService
}

func (fs *funImpl) OnStart() error {
	if fs.ops.Starter != nil {
		return fs.ops.Starter()
	}
	return nil
}

func (fs *funImpl) OnStop() {
	if fs.ops.Stopper != nil {
		fs.ops.Stopper()
	}
}

func (fs *funImpl) OnReset() error {
	if fs.ops.Reseter != nil {
		return fs.ops.Reseter()
	}
	return nil
}
