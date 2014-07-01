package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	//"crypto/rand"
	//"encoding/hex"
)

var APP_DIR = os.Getenv("HOME") + "/.tendermint"

/* Global & initialization */

var Config Config_

func init() {

	configFile := APP_DIR + "/config.json"

	// try to read configuration. if missing, write default
	configBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		defaultConfig.write(configFile)
		fmt.Println("Config file written to config.json. Please edit & run again")
		os.Exit(1)
		return
	}

	// try to parse configuration. on error, die
	Config = Config_{}
	err = json.Unmarshal(configBytes, &Config)
	if err != nil {
		log.Panicf("Invalid configuration file %s: %v", configFile, err)
	}
	err = Config.validate()
	if err != nil {
		log.Panicf("Invalid configuration file %s: %v", configFile, err)
	}
}

/* Default configuration */

var defaultConfig = Config_{
	Host: "127.0.0.1",
	Port: 8770,
	Db: DbConfig{
		Type: "level",
		Dir:  APP_DIR + "/data",
	},
	Twilio: TwilioConfig{},
}

/* Configuration types */

type Config_ struct {
	Host   string
	Port   int
	Db     DbConfig
	Twilio TwilioConfig
}

type TwilioConfig struct {
	Sid         string
	Token       string
	From        string
	To          string
	MinInterval int
}

type DbConfig struct {
	Type string
	Dir  string
}

func (cfg *Config_) validate() error {
	if cfg.Host == "" {
		return errors.New("Host must be set")
	}
	if cfg.Port == 0 {
		return errors.New("Port must be set")
	}
	if cfg.Db.Type == "" {
		return errors.New("Db.Type must be set")
	}
	return nil
}

func (cfg *Config_) bytes() []byte {
	configBytes, err := json.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	return configBytes
}

func (cfg *Config_) write(configFile string) {
	if strings.Index(configFile, "/") != -1 {
		err := os.MkdirAll(filepath.Dir(configFile), 0700)
		if err != nil {
			panic(err)
		}
	}
	err := ioutil.WriteFile(configFile, cfg.bytes(), 0600)
	if err != nil {
		panic(err)
	}
}

/* TODO: generate priv/pub keys
func generateKeys() string {
    bytes := &[30]byte{}
    rand.Read(bytes[:])
    return hex.EncodeToString(bytes[:])
}
*/
