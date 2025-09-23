package serverconfig

import (
	"flag"
	"fmt"
	"gometrics/internal/addr"

	"github.com/caarlos0/env/v6"
)

type ServerConfigs struct {
	Addr       addr.Addr `env:"ADDRESS" envDefault:"localhost:8080"`
	StoreInter int       `env:"STORE_INTERVAL" envDefault:"300"`
	FilePath   string    `env:"FILE_STORAGE_PATH" envDefault:"metrics_storage"`
	Restore    bool      `env:"RESTORE" envDefault:"false"`
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
	flag.IntVar(&o.StoreInter, "i", o.StoreInter, "Flush metrics interval")
	flag.StringVar(&o.FilePath, "f", o.FilePath, "Metrics store file destination")
	flag.BoolVar(&o.Restore, "r", o.Restore, "Restore metrics from json file")
	flag.Parse()
}
