package clientconfig

import (
	"flag"
	"fmt"

	"gometrics/internal/addr"

	"github.com/caarlos0/env/v6"
)

type ClientConfig struct {
	ReportInterval int       `env:"REPORT_INTERVAL" envDefault:"10"`
	PollInterval   int       `env:"POLL_INTERVAL" envDefault:"2"`
	Addr           addr.Addr `env:"ADDRESS" envDefault:"localhost:8080"`
	Compress       string    `env:"compress" envDefault:"gzip"`
	Key            string    `env:"KEY" envDefault:""`
	RateLimit      int       `env:"RATE_LIMIT" envDefault:"1"`
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

func (o *ClientConfig) ParseFlags() {
	if err := env.Parse(o); err != nil {
		fmt.Println("ENV var not found")
	}
	envKey := o.Key
	flag.IntVar(&o.ReportInterval, "r", o.ReportInterval, "Send to server interval")
	flag.IntVar(&o.PollInterval, "p", o.PollInterval, "Refresh metrics interval")
	flag.IntVar(&o.RateLimit, "l", o.RateLimit, "sender counter")
	flag.Var(&o.Addr, "a", "Host and port for connect/create")
	flag.StringVar(&o.Compress, "c", o.Compress, "Send metrics with compression")
	flag.StringVar(&o.Key, "k", o.Key, "Cipher key")
	flag.Parse()
	if envKey != "" {
		o.Key = envKey
	}
}
