package proxy

import (
	tmspcli "github.com/tendermint/tmsp/client/golang"
)

type AppConn interface {
	SetResponseCallback(tmspcli.Callback)
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
