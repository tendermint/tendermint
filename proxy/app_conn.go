package proxy

import (
	tmsp "github.com/tendermint/tmsp/types"
)

type Callback func(tmsp.Request, tmsp.Response)

type AppConn interface {
	Stop() bool
	IsRunning() bool
	SetResponseCallback(Callback)
	Error() error

	EchoAsync(msg string)
	FlushAsync()
	AppendTxAsync(tx []byte)
	CheckTxAsync(tx []byte)
	GetHashAsync()
	SetOptionAsync(key string, value string)
	AddListenerAsync(key string)
	RemListenerAsync(key string)

	InfoSync() (info []string, err error)
	FlushSync() error
	GetHashSync() (hash []byte, err error)
}
