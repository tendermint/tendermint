package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

func Prompt(prompt string, defaultValue string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		log.Warn("Error reading stdin", "err", err)
		return defaultValue
	} else {
		line = strings.TrimSpace(line)
		if line == "" {
			return defaultValue
		}
		return line
	}
}

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

type MapConfig map[string]interface{}

func (cfg MapConfig) Get(key string) interface{}    { return cfg[key] }
func (cfg MapConfig) GetBool(key string) bool       { return cfg[key].(bool) }
func (cfg MapConfig) GetFloat64(key string) float64 { return cfg[key].(float64) }
func (cfg MapConfig) GetInt(key string) int         { return cfg[key].(int) }
func (cfg MapConfig) GetString(key string) string   { return cfg[key].(string) }
func (cfg MapConfig) GetStringMap(key string) map[string]interface{} {
	return cfg[key].(map[string]interface{})
}
func (cfg MapConfig) GetStringMapString(key string) map[string]string {
	return cfg[key].(map[string]string)
}
func (cfg MapConfig) GetStringSlice(key string) []string { return cfg[key].([]string) }
func (cfg MapConfig) GetTime(key string) time.Time       { return cfg[key].(time.Time) }
func (cfg MapConfig) IsSet(key string) bool              { _, ok := cfg[key]; return ok }
func (cfg MapConfig) Set(key string, value interface{})  { cfg[key] = value }
func (cfg MapConfig) SetDefault(key string, value interface{}) {
	if cfg.IsSet(key) {
		return
	}
	cfg[key] = value
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
