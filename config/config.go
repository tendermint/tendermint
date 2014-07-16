package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	//"crypto/rand"
	//"encoding/hex"
)

/* Global & initialization */

var RootDir string
var Config Config_

func initFlags(printHelp *bool) {
	flag.BoolVar(printHelp, "help", false, "Print this help message.")
	flag.StringVar(&Config.LAddr, "laddr", Config.LAddr, "Listen address. (0.0.0.0:0 means any interface, any port)")
	flag.StringVar(&Config.Seed, "seed", Config.Seed, "Address of seed node")
}

func init() {
	RootDir = os.Getenv("TMROOT")
	if RootDir == "" {
		RootDir = os.Getenv("HOME") + "/.tendermint"
	}
	configFile := RootDir + "/config.json"

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

	// try to parse arg flags, which can override file configuration.
	var printHelp bool
	initFlags(&printHelp)
	flag.Parse()
	if printHelp {
		fmt.Println("----------------------------------")
		flag.PrintDefaults()
		fmt.Println("----------------------------------")
		os.Exit(0)
	}
}

/* Default configuration */

var defaultConfig = Config_{
	LAddr: "0.0.0.0:0",
	Seed:  "",
	Db: DbConfig{
		Type: "level",
		Dir:  RootDir + "/data",
	},
	Twilio: TwilioConfig{},
}

/* Configuration types */

type Config_ struct {
	LAddr  string
	Seed   string
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
	if cfg.LAddr == "" {
		cfg.LAddr = defaultConfig.LAddr
	}
	if cfg.Seed == "" {
		cfg.Seed = defaultConfig.Seed
	}
	if cfg.Db.Type == "" {
		return errors.New("Db.Type must be set")
	}
	return nil
}

func (cfg *Config_) bytes() []byte {
	configBytes, err := json.MarshalIndent(cfg, "", "\t")
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
