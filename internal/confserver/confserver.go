package confserver

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

type ConfigServer struct {
	Addr addr `env:"ADDRESS" envDefault:"localhost:8080"`
}

func (o *ConfigServer) ParseFlags() {
	env.Parse(o)
	o.Addr.AddrVar("server")
	flag.Parse()
}

func InitialFlags() ConfigServer {
	newInstance := ConfigServer{}
	return newInstance
}
