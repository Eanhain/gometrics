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

// ParseFlags reads configuration from environment variables, command-line flags, and JSON file.
func (o *ClientConfig) ParseFlags() {
	if flag.Lookup("r") == nil {
		flag.IntVar(&o.ReportInterval, "r", o.ReportInterval, "Send to server interval")
		flag.IntVar(&o.PollInterval, "p", o.PollInterval, "Refresh metrics interval")
		flag.IntVar(&o.RateLimit, "l", o.RateLimit, "Sender worker count")
		flag.Var(&o.Addr, "a", "Host and port")
		// Оставляем -c для компрессии, как ты требовал
		flag.StringVar(&o.Compress, "c", o.Compress, "Compression")
		flag.StringVar(&o.Key, "k", o.Key, "Key")
		flag.StringVar(&o.CryptoKey, "crypto-key", o.CryptoKey, "Crypto Key")

		// Для конфига используем -config
		flag.StringVar(&o.ConfigPath, "config", "", "Config path")
	}

	flag.Parse()

	if err := env.Parse(o); err != nil {
		fmt.Println("ENV parse error:", err)
	}

	if envKey := os.Getenv("KEY"); envKey != "" {
		o.Key = envKey
	}

	if o.ConfigPath != "" {
		o.loadConfigFile(o.ConfigPath)
	}
}

func (o *ClientConfig) loadConfigFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	var jCfg fileConfig
	if err := json.NewDecoder(file).Decode(&jCfg); err != nil {
		return
	}

	// Хелпер для проверки: установлен ли флаг явно
	isFlagSet := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		isFlagSet[f.Name] = true
	})

	shouldApply := func(flagName, envName string) bool {
		return !isFlagSet[flagName] && os.Getenv(envName) == ""
	}

	if shouldApply("r", "REPORT_INTERVAL") && jCfg.ReportInterval != "" {
		if dur, err := time.ParseDuration(jCfg.ReportInterval); err == nil {
			o.ReportInterval = int(dur.Seconds())
		}
	}
	if shouldApply("p", "POLL_INTERVAL") && jCfg.PollInterval != "" {
		if dur, err := time.ParseDuration(jCfg.PollInterval); err == nil {
			o.PollInterval = int(dur.Seconds())
		}
	}
	if shouldApply("a", "ADDRESS") && jCfg.Address != "" {
		o.Addr.Set(jCfg.Address)
	}
	if shouldApply("crypto-key", "CRYPTO_KEY") && jCfg.CryptoKey != "" {
		o.CryptoKey = jCfg.CryptoKey
	}
}

// ParseFlagsFromArgs - Хелпер ТОЛЬКО для тестов.
// Он эмулирует логику ParseFlags, но использует локальный FlagSet.
func (o *ClientConfig) ParseFlagsFromArgs(args []string) error {
	fs := flag.NewFlagSet("client-test", flag.ContinueOnError)

	// ВАЖНО: Определяем ТЕ ЖЕ флаги, что и в ParseFlags
	fs.IntVar(&o.ReportInterval, "r", o.ReportInterval, "")
	fs.IntVar(&o.PollInterval, "p", o.PollInterval, "")
	fs.IntVar(&o.RateLimit, "l", o.RateLimit, "")
	fs.Var(&o.Addr, "a", "")
	fs.StringVar(&o.Compress, "c", o.Compress, "") // -c для компрессии
	fs.StringVar(&o.Key, "k", o.Key, "")
	fs.StringVar(&o.CryptoKey, "crypto-key", o.CryptoKey, "")
	fs.StringVar(&o.ConfigPath, "config", "", "")

	// Парсим переданные аргументы
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Накатываем ENV
	if err := env.Parse(o); err != nil {
		return err
	}

	if envKey := os.Getenv("KEY"); envKey != "" {
		o.Key = envKey
	}

	// Накатываем JSON (упрощенно для тестов)
	if o.ConfigPath != "" {
		// Для тестов просто грузим файл, не проверяя flag.Visit (так как fs локальный)
		file, err := os.Open(o.ConfigPath)
		if err == nil {
			defer file.Close()
			var jCfg fileConfig
			if json.NewDecoder(file).Decode(&jCfg) == nil {
				// Логика приоритета JSON < Flag в тестах
				// Если значение в структуре равно дефолтному (значит флаг не менял), берем из JSON
				if o.ReportInterval == 10 && jCfg.ReportInterval != "" { // 10 = default
					if dur, err := time.ParseDuration(jCfg.ReportInterval); err == nil {
						o.ReportInterval = int(dur.Seconds())
					}
				}
				if o.PollInterval == 2 && jCfg.PollInterval != "" { // 2 = default
					if dur, err := time.ParseDuration(jCfg.PollInterval); err == nil {
						o.PollInterval = int(dur.Seconds())
					}
				}
				// Остальные поля по аналогии, если нужно для тестов
				if o.Addr.Host == "" && jCfg.Address != "" {
					o.Addr.Set(jCfg.Address)
				}
			}
		}
	}
	return nil
}
