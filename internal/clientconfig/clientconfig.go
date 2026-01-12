// Package clientconfig manages configuration parameters for the metrics client agent.
package clientconfig

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"gometrics/internal/addr"

	"github.com/caarlos0/env/v6"
)

// ClientConfig holds all configuration settings for the client.
type ClientConfig struct {
	ReportInterval int       `env:"REPORT_INTERVAL" envDefault:"10"`
	PollInterval   int       `env:"POLL_INTERVAL" envDefault:"2"`
	Addr           addr.Addr `env:"ADDRESS" envDefault:"localhost:8080"`
	Compress       string    `env:"COMPRESS" envDefault:"gzip"`
	Key            string    `env:"KEY" envDefault:""`
	RateLimit      int       `env:"RATE_LIMIT" envDefault:"5"`
	CryptoKey      string    `env:"CRYPTO_KEY" envDefault:""`
	ConfigPath     string    `env:"CONFIG"`
}

// fileConfig maps JSON fields.
type fileConfig struct {
	Address        string `json:"address"`
	ReportInterval string `json:"report_interval"`
	PollInterval   string `json:"poll_interval"`
	CryptoKey      string `json:"crypto_key"`
}

func (o *ClientConfig) GetPort() string {
	return fmt.Sprintf(":%d", o.Addr.GetPort())
}

func (o *ClientConfig) GetHost() string {
	return o.Addr.GetHost()
}

func InitialFlags() ClientConfig {
	return ClientConfig{
		Addr: addr.Addr{},
	}
}

// ParseFlags reads configuration.
func (o *ClientConfig) ParseFlags() {
	// 1. Define Flags
	if flag.Lookup("r") == nil {
		flag.IntVar(&o.ReportInterval, "r", o.ReportInterval, "Send to server interval")
		flag.IntVar(&o.PollInterval, "p", o.PollInterval, "Refresh metrics interval")
		flag.IntVar(&o.RateLimit, "l", o.RateLimit, "Sender worker count")
		flag.Var(&o.Addr, "a", "Host and port for connect/create")

		// ОСТАВЛЯЕМ -c ДЛЯ КОМПРЕССИИ (как вы просили)
		flag.StringVar(&o.Compress, "c", o.Compress, "Send metrics with compression")

		flag.StringVar(&o.Key, "k", o.Key, "Cipher key")
		flag.StringVar(&o.CryptoKey, "crypto-key", o.CryptoKey, "Public key for encryption")

		// Для конфига оставляем ТОЛЬКО -config, так как -c занят
		flag.StringVar(&o.ConfigPath, "config", "", "Path to configuration file")
	}

	flag.Parse()

	if err := env.Parse(o); err != nil {
		fmt.Println("ENV parse error:", err)
	}

	// Legacy Env override for Key
	envKey := os.Getenv("KEY")
	if envKey != "" {
		o.Key = envKey
	}

	// 4. Load JSON Config
	if o.ConfigPath != "" {
		o.loadConfigFile(o.ConfigPath)
	}
}

func (o *ClientConfig) loadConfigFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("Error opening config file: %v\n", err)
		return
	}
	defer file.Close()

	var jCfg fileConfig
	if err := json.NewDecoder(file).Decode(&jCfg); err != nil {
		fmt.Printf("Error decoding config file: %v\n", err)
		return
	}

	isFlagSet := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		isFlagSet[f.Name] = true
	})

	isReportSet := isFlagSet["r"] || os.Getenv("REPORT_INTERVAL") != ""
	isPollSet := isFlagSet["p"] || os.Getenv("POLL_INTERVAL") != ""
	isAddrSet := isFlagSet["a"] || os.Getenv("ADDRESS") != ""
	isCryptoSet := isFlagSet["crypto-key"] || os.Getenv("CRYPTO_KEY") != ""

	if !isReportSet && jCfg.ReportInterval != "" {
		if dur, err := time.ParseDuration(jCfg.ReportInterval); err == nil {
			o.ReportInterval = int(dur.Seconds())
		}
	}

	if !isPollSet && jCfg.PollInterval != "" {
		if dur, err := time.ParseDuration(jCfg.PollInterval); err == nil {
			o.PollInterval = int(dur.Seconds())
		}
	}

	if !isAddrSet && jCfg.Address != "" {
		o.Addr.Set(jCfg.Address)
	}

	if !isCryptoSet && jCfg.CryptoKey != "" {
		o.CryptoKey = jCfg.CryptoKey
	}
}

// ParseFlagsFromArgs - helper for tests
func (o *ClientConfig) ParseFlagsFromArgs(args []string) error {
	fs := flag.NewFlagSet("test-client", flag.ContinueOnError)

	fs.IntVar(&o.ReportInterval, "r", o.ReportInterval, "")
	fs.IntVar(&o.PollInterval, "p", o.PollInterval, "")
	fs.IntVar(&o.RateLimit, "l", o.RateLimit, "")
	fs.Var(&o.Addr, "a", "")

	// В тестах тоже мапим -c на Compress
	fs.StringVar(&o.Compress, "c", o.Compress, "")

	fs.StringVar(&o.Key, "k", o.Key, "")
	fs.StringVar(&o.CryptoKey, "crypto-key", o.CryptoKey, "")
	fs.StringVar(&o.ConfigPath, "config", "", "")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := env.Parse(o); err != nil {
		return err
	}

	if o.ConfigPath != "" {
		o.loadConfigFile(o.ConfigPath)
	}
	return nil
}
