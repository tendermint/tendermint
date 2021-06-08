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
