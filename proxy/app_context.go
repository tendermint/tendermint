package proxy

import (
	tmsp "github.com/tendermint/tmsp/types"
)

type Callback func(tmsp.Request, tmsp.Response)

type AppContext interface {
	SetResponseCallback(Callback)
	Error() error

	EchoAsync(msg string)
	FlushAsync()
	AppendTxAsync(tx []byte)
	GetHashAsync()
	CommitAsync()
	RollbackAsync()
	SetOptionAsync(key string, value string)
	AddListenerAsync(key string)
	RemListenerAsync(key string)

	InfoSync() (info []string, err error)
	FlushSync() error
	GetHashSync() (hash []byte, err error)
	CommitSync() error
	RollbackSync() error
}
