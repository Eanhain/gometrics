package serverflags

import (
	"flag"
	"fmt"

	"gometrics/internal/addr"

	"github.com/caarlos0/env/v6"
)

type ServerConfigs struct {
	Addr addr.Interface `env:"ADDRESS" envDefault:"localhost:8080"`
}

func (o *ServerConfigs) GetPort() string {
	return fmt.Sprintf(":%d", o.Addr.GetPort())
}

func (o *ServerConfigs) GetHost() string {
	return o.Addr.GetHost()
}

func InitialFlags() ServerConfigs {
	return ServerConfigs{
		Addr: &addr.Addr{},
	}
}

func (o *ServerConfigs) ParseFlags() {
	if err := env.Parse(o); err != nil {
		panic(err)
	}
	flag.Var(o.Addr, "a", "Host and port for connect/create")
	flag.Parse()
}
