package p2p

import (
	"github.com/cihub/seelog"
)

var log seelog.LoggerInterface

func SetLogger(l seelog.LoggerInterface) {
	log = l
}
