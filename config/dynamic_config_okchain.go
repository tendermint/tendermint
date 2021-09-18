package config

type IDynamicConfig interface {
	GetMempoolRecheck() bool
	GetMempoolForceRecheckGap() int64
	GetMempoolSize() int
}

var DynamicConfig IDynamicConfig = MockDynamicConfig{}

func SetDynamicConfig(c IDynamicConfig) {
	DynamicConfig = c
}

type MockDynamicConfig struct {
}

func (d MockDynamicConfig) GetMempoolRecheck() bool {
	return true
}

func (d MockDynamicConfig) GetMempoolForceRecheckGap() int64 {
	return 200
}

func (d MockDynamicConfig) GetMempoolSize() int {
	return 2000
}
