package config

type IDynamicConfig interface {
	GetMempoolRecheck() bool
	GetMempoolForceRecheckGap() int64
	GetMempoolSize() int
}

var DynamicConfig IDynamicConfig

func SetDynamicConfig(c IDynamicConfig) {
	DynamicConfig = c
}
