// Package serverconfig manages configuration parameters for the metrics server.
// It supports configuration via command-line flags, environment variables, and JSON config file,
// prioritizing flags over environment variables, and environment variables over config file.
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

// ServerConfigs holds all configuration settings for the server.
type ServerConfigs struct {
	// Addr represents the server address (host:port).
	Addr addr.Addr `env:"ADDRESS" envDefault:"localhost:8080"`

	// StoreInter is the frequency (in seconds) of flushing metrics to storage.
	StoreInter int `env:"STORE_INTERVAL" envDefault:"300"`

	// FilePath is the path to the file storage for metrics.
	FilePath string `env:"FILE_STORAGE_PATH" envDefault:"metrics_storage"`

	// Restore indicates whether to restore metrics from file on startup.
	Restore bool `env:"RESTORE" envDefault:"true"`

	// DatabaseDSN is the database connection string.
	DatabaseDSN string `env:"DATABASE_DSN" envDefault:""`

	// Key is the secret key for signing metrics data (SHA256).
	Key string `env:"KEY" envDefault:""`

	// CryptoKey is the path to the private key for HTTPS/encryption.
	CryptoKey string `env:"CRYPTO_KEY" envDefault:""`

	// ConfigPath is the path to the JSON configuration file.
	ConfigPath string `env:"CONFIG"`
}

// fileConfig is an internal struct to map JSON fields exactly as required.
type fileConfig struct {
	Address       string `json:"address"`
	Restore       bool   `json:"restore"`
	StoreInterval string `json:"store_interval"` // JSON uses string duration like "1s"
	StoreFile     string `json:"store_file"`
	DatabaseDSN   string `json:"database_dsn"`
	CryptoKey     string `json:"crypto_key"`
}

// GetPort returns the port string formatted with a colon (e.g., ":8080").
func (o *ServerConfigs) GetPort() string {
	return fmt.Sprintf(":%d", o.Addr.GetPort())
}

// GetHost returns the hostname part of the address.
func (o *ServerConfigs) GetHost() string {
	return o.Addr.GetHost()
}

// GetAddr returns the full address string.
func (o *ServerConfigs) GetAddr() string {
	return o.Addr.GetAddr()
}

// InitialFlags creates a new ServerConfigs with zero values.
func InitialFlags() ServerConfigs {
	return ServerConfigs{
		Addr: addr.Addr{},
	}
}

// ParseFlags reads configuration from environment variables, command-line flags, and JSON file.
// Priority: Flags > Env > Config File > Defaults.
func (o *ServerConfigs) ParseFlags() {
	// 1. Define Flags.
	if flag.Lookup("a") == nil {
		flag.Var(&o.Addr, "a", "Host and port for connect/create")
		flag.IntVar(&o.StoreInter, "i", o.StoreInter, "Flush metrics interval")
		flag.StringVar(&o.FilePath, "f", o.FilePath, "Metrics store file destination")
		flag.StringVar(&o.DatabaseDSN, "d", o.DatabaseDSN, "DB connection string")
		flag.StringVar(&o.Key, "k", o.Key, "Cipher key")
		flag.StringVar(&o.CryptoKey, "crypto-key", o.CryptoKey, "Private key for HTTPS")
		flag.BoolVar(&o.Restore, "r", o.Restore, "Restore metrics from json file")

		// Config path flags
		flag.StringVar(&o.ConfigPath, "config", "", "Path to configuration file")
	}

	// For compatibility with task requirement: -c/-config
	if flag.Lookup("c") == nil {
		flag.StringVar(&o.ConfigPath, "c", "", "Path to configuration file")
	}

	// 2. Parse Flags (fills struct with CLI values or Defaults)
	flag.Parse()

	// 3. Parse Env (overwrites struct with ENV values)
	if err := env.Parse(o); err != nil {
		fmt.Println("ENV parse error:", err)
	}

	// Special 'Key' logic from original code
	envKey := os.Getenv("KEY")
	if envKey != "" {
		o.Key = envKey
	}

	// 4. Load Config File (Lowest priority than Env/Flag, but overwrites Defaults)
	if o.ConfigPath != "" {
		o.loadConfigFile(o.ConfigPath)
	}
}

func (o *ServerConfigs) loadConfigFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("Error opening config file: %v\n", err)
		return
	}
	defer file.Close()

	var jCfg fileConfig
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&jCfg); err != nil {
		fmt.Printf("Error decoding config file: %v\n", err)
		return
	}

	// Helper maps to check if value was explicitly set by User (Flag or Env)
	isFlagSet := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		isFlagSet[f.Name] = true
	})

	// Check which values were set explicitly via flags or env vars
	isAddrSet := isFlagSet["a"] || os.Getenv("ADDRESS") != ""
	isRestoreSet := isFlagSet["r"] || os.Getenv("RESTORE") != ""
	isStoreIntervalSet := isFlagSet["i"] || os.Getenv("STORE_INTERVAL") != ""
	isStoreFileSet := isFlagSet["f"] || os.Getenv("FILE_STORAGE_PATH") != ""
	isDatabaseDSNSet := isFlagSet["d"] || os.Getenv("DATABASE_DSN") != ""
	isCryptoKeySet := isFlagSet["crypto-key"] || os.Getenv("CRYPTO_KEY") != ""

	// Apply JSON values if not set by higher priority sources

	// 1. Address
	if !isAddrSet && jCfg.Address != "" {
		if err := o.Addr.Set(jCfg.Address); err != nil {
			fmt.Printf("Invalid address in config file: %v\n", err)
		}
	}

	// 2. Restore
	if !isRestoreSet {
		// Note: JSON unmarshals bool directly, so we check if it was in file
		// Since Go bool has default false, and JSON can explicitly set true/false,
		// we apply it when flag/env not set
		o.Restore = jCfg.Restore
	}

	// 3. Store Interval
	if !isStoreIntervalSet && jCfg.StoreInterval != "" {
		dur, err := time.ParseDuration(jCfg.StoreInterval)
		if err == nil {
			o.StoreInter = int(dur.Seconds())
		} else {
			fmt.Printf("Invalid duration in config file for store_interval: %v\n", err)
		}
	}

	// 4. Store File
	if !isStoreFileSet && jCfg.StoreFile != "" {
		o.FilePath = jCfg.StoreFile
	}

	// 5. Database DSN
	if !isDatabaseDSNSet && jCfg.DatabaseDSN != "" {
		o.DatabaseDSN = jCfg.DatabaseDSN
	}

	// 6. Crypto Key
	if !isCryptoKeySet && jCfg.CryptoKey != "" {
		o.CryptoKey = jCfg.CryptoKey
	}
}
