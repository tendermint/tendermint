package config

import (
	"github.com/naoina/toml"
	"sync"
	"time"

	. "github.com/tendermint/go-common"
)

type Config interface {
	Get(key string) interface{}
	GetBool(key string) bool
	GetFloat64(key string) float64
	GetInt(key string) int
	GetString(key string) string
	GetStringMap(key string) map[string]interface{}
	GetStringMapString(key string) map[string]string
	GetStringSlice(key string) []string
	GetTime(key string) time.Time
	IsSet(key string) bool
	Set(key string, value interface{})
}

type MapConfig struct {
	required map[string]struct{} // blows up if trying to use before setting.
	data     map[string]interface{}
}

func ReadMapConfigFromFile(filePath string) (MapConfig, error) {
	var configData = make(map[string]interface{})
	fileBytes := MustReadFile(filePath)
	err := toml.Unmarshal(fileBytes, configData)
	if err != nil {
		return MapConfig{}, err
	}
	return NewMapConfig(configData), nil
}

func NewMapConfig(data map[string]interface{}) MapConfig {
	if data == nil {
		data = make(map[string]interface{})
	}
	return MapConfig{
		required: make(map[string]struct{}),
		data:     data,
	}
}

func (cfg MapConfig) Get(key string) interface{} {
	if _, ok := cfg.required[key]; ok {
		PanicSanity(Fmt("config key %v is required but was not set.", key))
	}
	return cfg.data[key]
}
func (cfg MapConfig) GetBool(key string) bool       { return cfg.Get(key).(bool) }
func (cfg MapConfig) GetFloat64(key string) float64 { return cfg.Get(key).(float64) }
func (cfg MapConfig) GetInt(key string) int         { return cfg.Get(key).(int) }
func (cfg MapConfig) GetString(key string) string   { return cfg.Get(key).(string) }
func (cfg MapConfig) GetStringMap(key string) map[string]interface{} {
	return cfg.Get(key).(map[string]interface{})
}
func (cfg MapConfig) GetStringMapString(key string) map[string]string {
	return cfg.Get(key).(map[string]string)
}
func (cfg MapConfig) GetStringSlice(key string) []string { return cfg.Get(key).([]string) }
func (cfg MapConfig) GetTime(key string) time.Time       { return cfg.Get(key).(time.Time) }
func (cfg MapConfig) IsSet(key string) bool              { _, ok := cfg.data[key]; return ok }
func (cfg MapConfig) Set(key string, value interface{}) {
	delete(cfg.required, key)
	cfg.data[key] = value
}
func (cfg MapConfig) SetDefault(key string, value interface{}) {
	delete(cfg.required, key)
	if cfg.IsSet(key) {
		return
	}
	cfg.data[key] = value
}
func (cfg MapConfig) SetRequired(key string) {
	if cfg.IsSet(key) {
		return
	}
	cfg.required[key] = struct{}{}
}

//--------------------------------------------------------------------------------
// A little convenient hack to notify listeners upon config changes.

type Configurable func(Config)

var mtx sync.Mutex
var globalConfig Config
var confs []Configurable

func OnConfig(conf func(Config)) {
	mtx.Lock()
	defer mtx.Unlock()

	confs = append(confs, conf)
	if globalConfig != nil {
		conf(globalConfig)
	}
}

func ApplyConfig(config Config) {
	mtx.Lock()
	globalConfig = config
	confsCopy := make([]Configurable, len(confs))
	copy(confsCopy, confs)
	mtx.Unlock()

	for _, conf := range confsCopy {
		conf(config)
	}
}
