package confapp

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/caarlos0/env/v6"
)

var ErrNotCorrect = errors.New("wrong host:port")

var err error

type Addr struct {
	Host string
	Port int
}

type Address struct {
	ReportInterval int  `env:"REPORT_INTERVAL" envDefault:"10"`
	PollInterval   int  `env:"POLL_INTERVAL" envDefault:"2"`
	Addr           Addr `env:"ADDRESS" envDefault:"localhost:8080"`
}

func (a *Addr) UnmarshalText(text []byte) error {
	address := string(text)
	address = strings.TrimSuffix(strings.TrimPrefix(address, "\""), "\"")
	err := a.Set(address)
	if err != nil {
		return err
	} else {
		return nil
	}
}

func (a *Addr) String() string {
	return fmt.Sprintf("%s:%v", a.Host, a.Port)
}

func (a *Addr) Set(flagValue string) error {
	args := strings.Split(flagValue, ":")
	a.Host = args[0]
	if len(args) == 0 || len(args) > 2 {
		return ErrNotCorrect
	}
	a.Port, err = strconv.Atoi(args[1])
	if err != nil {
		return ErrNotCorrect
	}
	return nil
}

func (o *Address) GetAddr() *Addr {
	return &o.Addr
}

func (o *Address) GetPort() string {
	return fmt.Sprintf(":%v", o.Addr.Port)
}

func (o *Address) GetHost() string {
	return o.Addr.Host

}

func InitialFlags() Address {
	newInstance := Address{}
	return newInstance
}

func (o *Address) ParseFlags(server bool) {
	env.Parse(o)
	if !server {
		flag.IntVar(&o.ReportInterval, "r", o.ReportInterval, "Send to server interval")
		flag.IntVar(&o.PollInterval, "p", o.PollInterval, "Refresh metrics interval")
		flag.Var(&o.Addr, "a", "Host and port for connect/create")
		flag.Parse()
	} else {
		flag.Var(&o.Addr, "a", "Host and port for connect/create")
		flag.Parse()
	}
}
