package confapp

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"
)

var ErrNotCorrect = errors.New("wrong host:port")

var err error

type Addr struct {
	Host string
	Port int
}

func (a *Addr) AddrVar(cmdType string) {
	descFormat := fmt.Sprintf("Create %v with that address", cmdType)
	flag.Var(a, "a", descFormat)
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

func (a *Addr) GetAddr() string {
	return a.String()
}

func (a *Addr) GetPort() string {
	return fmt.Sprintf(":%v", a.Port)
}

func (a *Addr) GetHost() string {
	return a.Host

}
