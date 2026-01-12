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

// JSONConfig структура для парсинга JSON файла конфигурации
type JSONConfig struct {
	Address       string `json:"address"`
	Restore       *bool  `json:"restore"`        // указатель для определения, было ли значение задано
	StoreInterval string `json:"store_interval"` // строка вида "1s", "300s" и т.д.
	StoreFile     string `json:"store_file"`
	DatabaseDSN   string `json:"database_dsn"`
	CryptoKey     string `json:"crypto_key"`
}

type ServerConfigs struct {
	Addr        addr.Addr `env:"ADDRESS" envDefault:"localhost:8080"`
	StoreInter  int       `env:"STORE_INTERVAL" envDefault:"300"`
	FilePath    string    `env:"FILE_STORAGE_PATH" envDefault:"metrics_storage"`
	Restore     bool      `env:"RESTORE" envDefault:"true"`
	DatabaseDSN string    `env:"DATABASE_DSN" envDefault:""`
	Key         string    `env:"KEY" envDefault:""`
	CryptoKey   string    `env:"CRYPTO_KEY" envDefault:""`
	ConfigPath  string    `env:"CONFIG" envDefault:""` // путь к JSON файлу конфигурации
}

func (o *ServerConfigs) GetPort() string {
	return fmt.Sprintf(":%d", o.Addr.GetPort())
}

func (o *ServerConfigs) GetHost() string {
	return o.Addr.GetHost()
}

func (o *ServerConfigs) GetAddr() string {
	return o.Addr.GetAddr()
}

func InitialFlags() ServerConfigs {
	return ServerConfigs{
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

func (o *ServerConfigs) ParseFlags() {
	// Шаг 1: Парсим переменные окружения (устанавливают значения по умолчанию)
	if err := env.Parse(o); err != nil {
		fmt.Println("env vars not found")
	}

	// Сохраняем значения из ENV для проверки приоритетов
	envKey := o.Key
	envConfigPath := o.ConfigPath

	// Шаг 2: Определяем флаги командной строки
	flag.Var(&o.Addr, "a", "Host and port for connect/create")
	flag.IntVar(&o.StoreInter, "i", o.StoreInter, "Flush metrics interval")
	flag.StringVar(&o.FilePath, "f", o.FilePath, "Metrics store file destination")
	flag.StringVar(&o.DatabaseDSN, "d", o.DatabaseDSN, "DB connection string")
	flag.StringVar(&o.Key, "k", o.Key, "Cipher key")
	flag.StringVar(&o.CryptoKey, "crypto-key", o.CryptoKey, "Public key for https")
	flag.BoolVar(&o.Restore, "r", o.Restore, "Restore metrics from json file")
	flag.StringVar(&o.ConfigPath, "config", o.ConfigPath, "Path to JSON config file")
	flag.StringVar(&o.ConfigPath, "c", o.ConfigPath, "Path to JSON config file (shorthand)")
	flag.Parse()

	// Шаг 3: Определяем путь к конфигу (ENV имеет приоритет над флагом для ConfigPath)
	configPath := o.ConfigPath
	if envConfigPath != "" {
		configPath = envConfigPath
	}

	// Шаг 4: Загружаем JSON конфигурацию (самый низкий приоритет)
	jsonCfg, err := loadJSONConfig(configPath)
	if err != nil {
		fmt.Printf("Warning: %v\n", err)
	}

	// Шаг 5: Применяем значения из JSON только если они не были установлены через флаги/env
	if jsonCfg != nil {
		o.applyJSONConfig(jsonCfg)
	}

	// Шаг 6: Особая логика для Key - ENV имеет приоритет над флагом
	if envKey != "" {
		o.Key = envKey
	}
}

// applyJSONConfig применяет значения из JSON конфига с учётом приоритетов
// JSON имеет самый низкий приоритет, поэтому применяется только к значениям по умолчанию
func (o *ServerConfigs) applyJSONConfig(cfg *JSONConfig) {
	// Проверяем, были ли установлены значения через флаги/env
	// Используем flag.Visit для определения, был ли флаг явно указан

	// Address
	if cfg.Address != "" && !isFlagPassed("a") && os.Getenv("ADDRESS") == "" {
		if err := o.Addr.Set(cfg.Address); err != nil {
			fmt.Printf("Warning: invalid address in config: %v\n", err)
		}
	}

	// StoreInterval
	if cfg.StoreInterval != "" && !isFlagPassed("i") && os.Getenv("STORE_INTERVAL") == "" {
		if interval, err := parseInterval(cfg.StoreInterval); err == nil {
			o.StoreInter = interval
		} else {
			fmt.Printf("Warning: invalid store_interval in config: %v\n", err)
		}
	}

	// StoreFile (FilePath)
	if cfg.StoreFile != "" && !isFlagPassed("f") && os.Getenv("FILE_STORAGE_PATH") == "" {
		o.FilePath = cfg.StoreFile
	}

	// Restore
	if cfg.Restore != nil && !isFlagPassed("r") && os.Getenv("RESTORE") == "" {
		o.Restore = *cfg.Restore
	}

	// DatabaseDSN
	if cfg.DatabaseDSN != "" && !isFlagPassed("d") && os.Getenv("DATABASE_DSN") == "" {
		o.DatabaseDSN = cfg.DatabaseDSN
	}

	// CryptoKey
	if cfg.CryptoKey != "" && !isFlagPassed("crypto-key") && os.Getenv("CRYPTO_KEY") == "" {
		o.CryptoKey = cfg.CryptoKey
	}
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
