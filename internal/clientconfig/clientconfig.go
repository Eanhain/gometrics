// Package clientconfig manages configuration parameters for the metrics client agent.
// It supports configuration via command-line flags, environment variables, and JSON config file,
// prioritizing: flags > environment variables > JSON config > default values
// (with specific logic for the 'Key' field where ENV has priority over flag).
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

// JSONConfig структура для парсинга JSON файла конфигурации агента
type JSONConfig struct {
	Address        string `json:"address"`         // аналог переменной окружения ADDRESS или флага -a
	ReportInterval string `json:"report_interval"` // аналог переменной окружения REPORT_INTERVAL или флага -r
	PollInterval   string `json:"poll_interval"`   // аналог переменной окружения POLL_INTERVAL или флага -p
	CryptoKey      string `json:"crypto_key"`      // аналог переменной окружения CRYPTO_KEY или флага -crypto-key
}

// ClientConfig holds all configuration settings for the client.
// Fields are tagged for parsing from environment variables using github.com/caarlos0/env.
type ClientConfig struct {
	// ReportInterval is the frequency (in seconds) of sending metrics to the server.
	ReportInterval int `env:"REPORT_INTERVAL" envDefault:"10"`

	// PollInterval is the frequency (in seconds) of updating metrics from runtime app.
	PollInterval int `env:"POLL_INTERVAL" envDefault:"2"`

	// Addr represents the target server address (host:port).
	Addr addr.Addr `env:"ADDRESS" envDefault:"localhost:8080"`

	// Compress defines the compression algorithm (e.g., "gzip").
	Compress string `env:"compress" envDefault:"gzip"`

	// Key is the secret key for signing metrics data (SHA256).
	Key string `env:"KEY" envDefault:""`

	// RateLimit controls the number of concurrent workers for sending metrics.
	RateLimit int `env:"RATE_LIMIT" envDefault:"5"`

	// CryptoKey is the path to public key for payload encryption.
	CryptoKey string `env:"CRYPTO_KEY" envDefault:""`

	// ConfigPath is the path to JSON configuration file.
	ConfigPath string `env:"CONFIG" envDefault:""`

	GRPCAddr string `env:"GRPC_ADDRESS" envDefault:""`  // адрес gRPC сервера
	UseGRPC  bool   `env:"USE_GRPC" envDefault:"false"` // использовать gRPC вместо
}

// GetPort returns the port string formatted with a colon (e.g., ":8080").
func (o *ClientConfig) GetPort() string {
	return fmt.Sprintf(":%d", o.Addr.GetPort())
}

// GetHost returns the hostname part of the address.
func (o *ClientConfig) GetHost() string {
	return o.Addr.GetHost()
}

// InitialFlags creates a new ClientConfig with zero values.
// Note: Default values are actually populated during env.Parse or flag definition.
func InitialFlags() ClientConfig {
	return ClientConfig{
		Addr: addr.Addr{},
	}
}

// loadJSONConfig загружает конфигурацию из JSON файла
func loadJSONConfig(path string) (*JSONConfig, error) {
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg JSONConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// parseInterval парсит строку интервала вида "1s", "5m" и возвращает секунды
func parseInterval(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	return int(d.Seconds()), nil
}

// isFlagPassed проверяет, был ли флаг явно передан в командной строке
func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

// isFlagPassedInSet проверяет, был ли флаг явно передан в FlagSet
func isFlagPassedInSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

// ParseFlags reads configuration from environment variables, JSON config file, and command-line flags.
//
// Priority logic:
// 1. Command-line flags (highest priority).
// 2. Environment variables.
// 3. JSON config file.
// 4. Default values (defined in struct tags and flag defaults).
//
// Special case for 'Key': The environment variable 'KEY' takes precedence over the flag '-k'
// if both are present and the env var is not empty (as per original logic).
func (o *ClientConfig) ParseFlags() {
	// 1. Parse Environment variables first to populate defaults or env-set values
	if err := env.Parse(o); err != nil {
		// In a real app, you might want to log this properly or return error
		fmt.Println("ENV var not found or parse error:", err)
	}

	// Capture the key from ENV if it exists, to restore it later if needed
	envKey := o.Key
	envConfigPath := o.ConfigPath

	// 2. Define Flags.
	// We use current values of 'o' (set by env or defaults) as default values for flags.
	// This allows flags to override env vars generally.
	// Since flag.Parse uses the global flag set, this should ideally be called once.
	if flag.Lookup("r") == nil { // Prevent re-definition in tests if running multiple times in same process
		flag.IntVar(&o.ReportInterval, "r", o.ReportInterval, "Send to server interval")
		flag.IntVar(&o.PollInterval, "p", o.PollInterval, "Refresh metrics interval")
		flag.IntVar(&o.RateLimit, "l", o.RateLimit, "sender counter")
		flag.Var(&o.Addr, "a", "Host and port for connect/create")
		flag.StringVar(&o.Compress, "c", o.Compress, "Send metrics with compression")
		flag.StringVar(&o.Key, "k", o.Key, "Cipher key")
		flag.StringVar(&o.CryptoKey, "crypto-key", o.CryptoKey, "Public key for payload encryption")
		flag.StringVar(&o.ConfigPath, "config", o.ConfigPath, "Path to JSON config file")
		flag.StringVar(&o.GRPCAddr, "grpc", o.GRPCAddr, "gRPC server address (e.g., localhost:3200)")
		flag.BoolVar(&o.UseGRPC, "use-grpc", o.UseGRPC, "Use gRPC instead of HTTP")
	}

	// 3. Parse Flags
	flag.Parse()

	// 4. Determine config path (ENV has priority for ConfigPath)
	configPath := o.ConfigPath
	if envConfigPath != "" {
		configPath = envConfigPath
	}

	// 5. Load JSON configuration (lowest priority)
	jsonCfg, err := loadJSONConfig(configPath)
	if err != nil {
		fmt.Printf("Warning: %v\n", err)
	}

	// 6. Apply JSON config values only if not set by flags/env
	if jsonCfg != nil {
		o.applyJSONConfig(jsonCfg)
	}

	// 7. Special Logic: Restore Key from Env if it was present.
	// The original code implies that ENV 'KEY' > FLAG 'k'.
	if envKey != "" {
		o.Key = envKey
	}
}

// applyJSONConfig применяет значения из JSON конфига с учётом приоритетов
// JSON имеет самый низкий приоритет, поэтому применяется только если значение не задано через флаги/env
func (o *ClientConfig) applyJSONConfig(cfg *JSONConfig) {
	// Address
	if cfg.Address != "" && !isFlagPassed("a") && os.Getenv("ADDRESS") == "" {
		if err := o.Addr.Set(cfg.Address); err != nil {
			fmt.Printf("Warning: invalid address in config: %v\n", err)
		}
	}

	// ReportInterval
	if cfg.ReportInterval != "" && !isFlagPassed("r") && os.Getenv("REPORT_INTERVAL") == "" {
		if interval, err := parseInterval(cfg.ReportInterval); err == nil {
			o.ReportInterval = interval
		} else {
			fmt.Printf("Warning: invalid report_interval in config: %v\n", err)
		}
	}

	// PollInterval
	if cfg.PollInterval != "" && !isFlagPassed("p") && os.Getenv("POLL_INTERVAL") == "" {
		if interval, err := parseInterval(cfg.PollInterval); err == nil {
			o.PollInterval = interval
		} else {
			fmt.Printf("Warning: invalid poll_interval in config: %v\n", err)
		}
	}

	// CryptoKey
	if cfg.CryptoKey != "" && !isFlagPassed("crypto-key") && os.Getenv("CRYPTO_KEY") == "" {
		o.CryptoKey = cfg.CryptoKey
	}
}

// applyJSONConfigFromSet применяет значения из JSON конфига для FlagSet (используется в тестах)
func (o *ClientConfig) applyJSONConfigFromSet(cfg *JSONConfig, fs *flag.FlagSet) {
	// Address
	if cfg.Address != "" && !isFlagPassedInSet(fs, "a") && os.Getenv("ADDRESS") == "" {
		if err := o.Addr.Set(cfg.Address); err != nil {
			fmt.Printf("Warning: invalid address in config: %v\n", err)
		}
	}

	// ReportInterval
	if cfg.ReportInterval != "" && !isFlagPassedInSet(fs, "r") && os.Getenv("REPORT_INTERVAL") == "" {
		if interval, err := parseInterval(cfg.ReportInterval); err == nil {
			o.ReportInterval = interval
		} else {
			fmt.Printf("Warning: invalid report_interval in config: %v\n", err)
		}
	}

	// PollInterval
	if cfg.PollInterval != "" && !isFlagPassedInSet(fs, "p") && os.Getenv("POLL_INTERVAL") == "" {
		if interval, err := parseInterval(cfg.PollInterval); err == nil {
			o.PollInterval = interval
		} else {
			fmt.Printf("Warning: invalid poll_interval in config: %v\n", err)
		}
	}

	// CryptoKey
	if cfg.CryptoKey != "" && !isFlagPassedInSet(fs, "crypto-key") && os.Getenv("CRYPTO_KEY") == "" {
		o.CryptoKey = cfg.CryptoKey
	}
}

// ParseFlagsFromArgs is a helper for testing that allows passing custom arguments.
// It mimics ParseFlags but works with a custom FlagSet.
func (o *ClientConfig) ParseFlagsFromArgs(args []string) error {
	// Reset to defaults or env
	if err := env.Parse(o); err != nil {
		return err
	}
	envKey := o.Key
	envConfigPath := o.ConfigPath

	fs := flag.NewFlagSet("test-client", flag.ContinueOnError)
	fs.IntVar(&o.ReportInterval, "r", o.ReportInterval, "Send to server interval")
	fs.IntVar(&o.PollInterval, "p", o.PollInterval, "Refresh metrics interval")
	fs.IntVar(&o.RateLimit, "l", o.RateLimit, "sender counter")
	fs.Var(&o.Addr, "a", "Host and port for connect/create")
	fs.StringVar(&o.Compress, "c", o.Compress, "Send metrics with compression")
	fs.StringVar(&o.Key, "k", o.Key, "Cipher key")
	fs.StringVar(&o.CryptoKey, "crypto-key", o.CryptoKey, "Public key for payload encryption")
	fs.StringVar(&o.ConfigPath, "config", o.ConfigPath, "Path to JSON config file")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Determine config path (ENV has priority for ConfigPath)
	configPath := o.ConfigPath
	if envConfigPath != "" {
		configPath = envConfigPath
	}

	// Load JSON configuration (lowest priority)
	jsonCfg, err := loadJSONConfig(configPath)
	if err != nil {
		return err
	}

	// Apply JSON config values only if not set by flags/env
	if jsonCfg != nil {
		o.applyJSONConfigFromSet(jsonCfg, fs)
	}

	if envKey != "" {
		o.Key = envKey
	}
	return nil
}
