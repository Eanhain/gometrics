package serverconfig

import (
	"flag"
	"fmt"
	"gometrics/internal/addr"

	"github.com/caarlos0/env/v6"
)

type ServerConfigs struct {
	Addr addr.Addr `env:"ADDRESS" envDefault:"localhost:8080"`
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

func (o *ServerConfigs) ParseFlags() {
	if err := env.Parse(o); err != nil {
		fmt.Println("env vars not found")
	}
	flag.Var(&o.Addr, "a", "Host and port for connect/create")
	flag.Parse()
}
