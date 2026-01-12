package serverconfig

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"gometrics/internal/addr"

	"github.com/caarlos0/env/v6"
)

type ServerConfigs struct {
	Addr        addr.Addr `env:"ADDRESS"`
	StoreInter  int       `env:"STORE_INTERVAL"`
	FilePath    string    `env:"FILE_STORAGE_PATH"`
	Restore     bool      `env:"RESTORE"`
	DatabaseDSN string    `env:"DATABASE_DSN"`
	Key         string    `env:"KEY"`
	CryptoKey   string    `env:"CRYPTO_KEY"`
	ConfigPath  string    `env:"CONFIG"`
}

type fileConfig struct {
	Address       string `json:"address"`
	Restore       bool   `json:"restore"`
	StoreInterval string `json:"store_interval"`
	StoreFile     string `json:"store_file"`
	DatabaseDSN   string `json:"database_dsn"`
	CryptoKey     string `json:"crypto_key"`
}

func InitialFlags() ServerConfigs {
	return ServerConfigs{
		Addr:       addr.Addr{Host: "localhost", Port: 8080},
		StoreInter: 300,
		FilePath:   "metrics_storage",
		Restore:    true,
	}
}

func (o *ServerConfigs) GetPort() string { return fmt.Sprintf(":%d", o.Addr.GetPort()) }
func (o *ServerConfigs) GetHost() string { return o.Addr.GetHost() }
func (o *ServerConfigs) GetAddr() string { return o.Addr.GetAddr() }

func (o *ServerConfigs) ParseFlags() {
	// 1. Env
	if err := env.Parse(o); err != nil {
		fmt.Println("ENV error:", err)
	}
	if envKey := os.Getenv("KEY"); envKey != "" {
		o.Key = envKey
	}

	// 2. Flags definition (using current values as defaults)
	if flag.Lookup("a") == nil {
		flag.Var(&o.Addr, "a", "Address")
		flag.IntVar(&o.StoreInter, "i", o.StoreInter, "Interval")
		flag.StringVar(&o.FilePath, "f", o.FilePath, "File")
		flag.StringVar(&o.DatabaseDSN, "d", o.DatabaseDSN, "DB")
		flag.StringVar(&o.Key, "k", o.Key, "Key")
		flag.StringVar(&o.CryptoKey, "crypto-key", o.CryptoKey, "Crypto")
		flag.BoolVar(&o.Restore, "r", o.Restore, "Restore")
		flag.StringVar(&o.ConfigPath, "config", "", "Config")
		flag.StringVar(&o.ConfigPath, "c", "", "Config alias")
	}

	// 3. Flags parse
	flag.Parse()

	// 4. JSON
	if o.ConfigPath != "" {
		o.loadConfigFile(o.ConfigPath)
	}
}

func (o *ServerConfigs) loadConfigFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	var jCfg fileConfig
	if json.NewDecoder(file).Decode(&jCfg) != nil {
		return
	}

	isFlagSet := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { isFlagSet[f.Name] = true })

	shouldApply := func(flagName, envName string) bool {
		return !isFlagSet[flagName] && os.Getenv(envName) == ""
	}

	if shouldApply("a", "ADDRESS") && jCfg.Address != "" {
		o.Addr.Set(jCfg.Address)
	}
	if shouldApply("r", "RESTORE") {
		// Bool хитрее, так как false - тоже значение.
		// Но в JSON если restore: true или false, оно применится.
		// Если в JSON нет поля, restore=false (zero value).
		// Тут логика упрощенная: если конфиг есть, верим ему, если флага нет.
		o.Restore = jCfg.Restore
	}
	if shouldApply("i", "STORE_INTERVAL") && jCfg.StoreInterval != "" {
		if dur, err := time.ParseDuration(jCfg.StoreInterval); err == nil {
			o.StoreInter = int(dur.Seconds())
		}
	}
	if shouldApply("f", "FILE_STORAGE_PATH") && jCfg.StoreFile != "" {
		o.FilePath = jCfg.StoreFile
	}
	if shouldApply("d", "DATABASE_DSN") && jCfg.DatabaseDSN != "" {
		o.DatabaseDSN = jCfg.DatabaseDSN
	}
	if shouldApply("crypto-key", "CRYPTO_KEY") && jCfg.CryptoKey != "" {
		o.CryptoKey = jCfg.CryptoKey
	}
}
