package serverconfig

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"gometrics/internal/addr"

	"github.com/caarlos0/env/v6"
)

// JSONConfig структура для парсинга JSON файла конфигурации сервера.
// Поддерживает все опции, аналогичные флагам и переменным окружения.
type JSONConfig struct {
	Address       string `json:"address"`        // аналог переменной окружения ADDRESS или флага -a
	Restore       *bool  `json:"restore"`        // аналог RESTORE или -r
	StoreInterval string `json:"store_interval"` // аналог STORE_INTERVAL или -i
	StoreFile     string `json:"store_file"`     // аналог FILE_STORAGE_PATH или -f
	DatabaseDSN   string `json:"database_dsn"`   // аналог DATABASE_DSN или -d
	CryptoKey     string `json:"crypto_key"`     // аналог CRYPTO_KEY или -crypto-key
	TrustedSubnet string `json:"trusted_subnet"` // аналог TRUSTED_SUBNET или -t (CIDR нотация)
}

// ServerConfigs содержит все настройки конфигурации сервера.
type ServerConfigs struct {
	Addr          addr.Addr `env:"ADDRESS" envDefault:"localhost:8080"`
	StoreInter    int       `env:"STORE_INTERVAL" envDefault:"300"`
	FilePath      string    `env:"FILE_STORAGE_PATH" envDefault:"metrics_storage"`
	Restore       bool      `env:"RESTORE" envDefault:"true"`
	DatabaseDSN   string    `env:"DATABASE_DSN" envDefault:""`
	Key           string    `env:"KEY" envDefault:""`
	CryptoKey     string    `env:"CRYPTO_KEY" envDefault:""`
	ConfigPath    string    `env:"CONFIG" envDefault:""`
	TrustedSubnet string    `env:"TRUSTED_SUBNET" envDefault:""` // CIDR нотация доверенной подсети
}

// GetPort возвращает порт в формате ":8080"
func (o *ServerConfigs) GetPort() string {
	return fmt.Sprintf(":%d", o.Addr.GetPort())
}

// GetHost возвращает хост из адреса
func (o *ServerConfigs) GetHost() string {
	return o.Addr.GetHost()
}

// GetAddr возвращает полный адрес в формате "host:port"
func (o *ServerConfigs) GetAddr() string {
	return o.Addr.GetAddr()
}

// InitialFlags создаёт новый экземпляр ServerConfigs с нулевыми значениями.
func InitialFlags() ServerConfigs {
	return ServerConfigs{Addr: addr.Addr{}}
}

// loadJSONConfig загружает конфигурацию из JSON файла.
// Возвращает nil, nil если путь пустой.
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

// parseInterval парсит строку интервала ("1s", "5m", "1h") и возвращает секунды.
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

// isFlagPassed проверяет, был ли флаг явно передан в командной строке.
func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

// isFlagPassedInSet проверяет, был ли флаг явно передан в указанном FlagSet.
func isFlagPassedInSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

// ParseFlags читает конфигурацию из env, JSON файла и флагов командной строки.
//
// Приоритет (от высшего к низшему):
//  1. Флаги командной строки
//  2. Переменные окружения
//  3. JSON файл конфигурации
//  4. Значения по умолчанию
//
// Исключение: для Key переменная окружения KEY имеет приоритет над флагом -k.
// ParseFlags читает конфигурацию из env, JSON файла и флагов командной строки.
func (o *ServerConfigs) ParseFlags() {
	if err := env.Parse(o); err != nil {
		fmt.Println("env vars not found")
	}

	envKey := o.Key
	envConfigPath := o.ConfigPath

	flag.Var(&o.Addr, "a", "Host and port for connect/create")
	flag.IntVar(&o.StoreInter, "i", o.StoreInter, "Flush metrics interval")
	flag.StringVar(&o.FilePath, "f", o.FilePath, "Metrics store file destination")
	flag.StringVar(&o.DatabaseDSN, "d", o.DatabaseDSN, "DB connection string")
	flag.StringVar(&o.Key, "k", o.Key, "Cipher key")
	flag.StringVar(&o.CryptoKey, "crypto-key", o.CryptoKey, "Public key for https")
	flag.BoolVar(&o.Restore, "r", o.Restore, "Restore metrics from json file")
	flag.StringVar(&o.ConfigPath, "config", o.ConfigPath, "Path to JSON config file")
	flag.StringVar(&o.ConfigPath, "c", o.ConfigPath, "Path to JSON config file (shorthand)")
	// Новый флаг для trusted subnet в CIDR нотации
	flag.StringVar(&o.TrustedSubnet, "t", o.TrustedSubnet, "Trusted subnet in CIDR notation (e.g., 192.168.1.0/24)")
	flag.Parse()

	configPath := o.ConfigPath
	if envConfigPath != "" {
		configPath = envConfigPath
	}

	jsonCfg, err := loadJSONConfig(configPath)
	if err != nil {
		fmt.Printf("Warning: %v\n", err)
	}

	if jsonCfg != nil {
		o.applyJSONConfig(jsonCfg)
	}

	if envKey != "" {
		o.Key = envKey
	}
}

// applyJSONConfig применяет значения из JSON конфига.
// Значение применяется только если флаг НЕ передан и env НЕ установлен.
func (o *ServerConfigs) applyJSONConfig(cfg *JSONConfig) {
	if cfg.Address != "" && !isFlagPassed("a") && os.Getenv("ADDRESS") == "" {
		_ = o.Addr.Set(cfg.Address)
	}
	if cfg.StoreInterval != "" && !isFlagPassed("i") && os.Getenv("STORE_INTERVAL") == "" {
		if interval, err := parseInterval(cfg.StoreInterval); err == nil {
			o.StoreInter = interval
		}
	}
	if cfg.StoreFile != "" && !isFlagPassed("f") && os.Getenv("FILE_STORAGE_PATH") == "" {
		o.FilePath = cfg.StoreFile
	}
	if cfg.Restore != nil && !isFlagPassed("r") && os.Getenv("RESTORE") == "" {
		o.Restore = *cfg.Restore
	}
	if cfg.DatabaseDSN != "" && !isFlagPassed("d") && os.Getenv("DATABASE_DSN") == "" {
		o.DatabaseDSN = cfg.DatabaseDSN
	}
	if cfg.CryptoKey != "" && !isFlagPassed("crypto-key") && os.Getenv("CRYPTO_KEY") == "" {
		o.CryptoKey = cfg.CryptoKey
	}
	// Применяем trusted_subnet из JSON если не задан через флаг или ENV
	if cfg.TrustedSubnet != "" && !isFlagPassed("t") && os.Getenv("TRUSTED_SUBNET") == "" {
		o.TrustedSubnet = cfg.TrustedSubnet
	}
}

// GetTrustedSubnet возвращает распарсенную подсеть или nil если не задана/невалидна
func (o *ServerConfigs) GetTrustedSubnet() *net.IPNet {
	if o.TrustedSubnet == "" {
		return nil
	}
	_, ipNet, err := net.ParseCIDR(o.TrustedSubnet)
	if err != nil {
		return nil
	}
	return ipNet
}

// applyJSONConfigFromSet применяет значения из JSON конфига для указанного FlagSet.
// Используется в тестах с отдельным FlagSet.
func (o *ServerConfigs) applyJSONConfigFromSet(cfg *JSONConfig, fs *flag.FlagSet) {
	if cfg.Address != "" && !isFlagPassedInSet(fs, "a") && os.Getenv("ADDRESS") == "" {
		_ = o.Addr.Set(cfg.Address)
	}
	if cfg.StoreInterval != "" && !isFlagPassedInSet(fs, "i") && os.Getenv("STORE_INTERVAL") == "" {
		if interval, err := parseInterval(cfg.StoreInterval); err == nil {
			o.StoreInter = interval
		}
	}
	if cfg.StoreFile != "" && !isFlagPassedInSet(fs, "f") && os.Getenv("FILE_STORAGE_PATH") == "" {
		o.FilePath = cfg.StoreFile
	}
	if cfg.Restore != nil && !isFlagPassedInSet(fs, "r") && os.Getenv("RESTORE") == "" {
		o.Restore = *cfg.Restore
	}
	if cfg.DatabaseDSN != "" && !isFlagPassedInSet(fs, "d") && os.Getenv("DATABASE_DSN") == "" {
		o.DatabaseDSN = cfg.DatabaseDSN
	}
	if cfg.CryptoKey != "" && !isFlagPassedInSet(fs, "crypto-key") && os.Getenv("CRYPTO_KEY") == "" {
		o.CryptoKey = cfg.CryptoKey
	}
}

// ParseFlagsFromArgs - хелпер для тестирования с кастомными аргументами.
// Работает аналогично ParseFlags, но использует отдельный FlagSet.
func (o *ServerConfigs) ParseFlagsFromArgs(args []string) error {
	if err := env.Parse(o); err != nil {
		return err
	}

	envKey := o.Key
	envConfigPath := o.ConfigPath

	fs := flag.NewFlagSet("test-server", flag.ContinueOnError)
	fs.Var(&o.Addr, "a", "Host and port for connect/create")
	fs.IntVar(&o.StoreInter, "i", o.StoreInter, "Flush metrics interval")
	fs.StringVar(&o.FilePath, "f", o.FilePath, "Metrics store file destination")
	fs.StringVar(&o.DatabaseDSN, "d", o.DatabaseDSN, "DB connection string")
	fs.StringVar(&o.Key, "k", o.Key, "Cipher key")
	fs.StringVar(&o.CryptoKey, "crypto-key", o.CryptoKey, "Public key for https")
	fs.BoolVar(&o.Restore, "r", o.Restore, "Restore metrics from json file")
	fs.StringVar(&o.ConfigPath, "config", o.ConfigPath, "Path to JSON config file")
	fs.StringVar(&o.ConfigPath, "c", o.ConfigPath, "Path to JSON config file (shorthand)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	configPath := o.ConfigPath
	if envConfigPath != "" {
		configPath = envConfigPath
	}

	jsonCfg, err := loadJSONConfig(configPath)
	if err != nil {
		return err
	}

	if jsonCfg != nil {
		o.applyJSONConfigFromSet(jsonCfg, fs)
	}

	if envKey != "" {
		o.Key = envKey
	}

	return nil
}
