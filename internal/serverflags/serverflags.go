package serverflags

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

func (a *Addr) UnmarshalText(text []byte) error {
	address := string(text)
	address = strings.TrimSuffix(strings.TrimPrefix(address, "\""), "\"")
	err := a.Set(address)
	if err != nil {
		return err
	}
	return nil

}

type ServerConfigs struct {
	Addr Addr `env:"ADDRESS" envDefault:"localhost:8080"`
}

func (a *Addr) String() string {
	return fmt.Sprintf("%s:%v", a.Host, a.Port)
}

func (a *Addr) Set(flagValue string) error {
	args := strings.Split(flagValue, ":")
	if len(args) == 0 || len(args) > 2 {
		return ErrNotCorrect
	}
	a.Host = args[0]
	a.Port, err = strconv.Atoi(args[1])
	if err != nil {
		return ErrNotCorrect
	}
	return nil
}

func (o *ServerConfigs) GetAddr() *Addr {
	return &o.Addr
}

func (o *ServerConfigs) GetPort() string {
	return fmt.Sprintf(":%v", o.Addr.Port)
}

func (o *ServerConfigs) GetHost() string {
	return o.Addr.Host

}

func InitialFlags() ServerConfigs {
	newInstance := ServerConfigs{}
	return newInstance
}

func (o *ServerConfigs) ParseFlags() {
	err := env.Parse(o)
	if err != nil {
		panic(err)
	}
	flag.Var(&o.Addr, "a", "Host and port for connect/create")
	flag.Parse()
}
