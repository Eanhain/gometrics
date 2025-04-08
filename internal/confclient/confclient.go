package confclient

import (
	"flag"

	"github.com/caarlos0/env/v6"
)

type addr interface {
	UnmarshalText(text []byte) error
	String() string
	Set(flagValue string) error
	GetAddr() string
	GetPort() string
	GetHost() string
	AddrVar(cmdType string)
}

type ConfigClient struct {
	ReportInterval int  `env:"REPORT_INTERVAL" envDefault:"10"`
	PollInterval   int  `env:"POLL_INTERVAL" envDefault:"2"`
	Addr           addr `env:"ADDRESS" envDefault:"localhost:8080"`
}

func (o *ConfigClient) ParseFlags() {
	env.Parse(o)
	flag.IntVar(&o.ReportInterval, "r", o.ReportInterval, "Send to server interval")
	flag.IntVar(&o.PollInterval, "p", o.PollInterval, "Refresh metrics interval")
	o.Addr.AddrVar("client")
	flag.Parse()
}

func InitialFlags() ConfigClient {
	newInstance := ConfigClient{}
	return newInstance
}
