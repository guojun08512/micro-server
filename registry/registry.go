package registry

import (
	"github.com/micro/go-micro/v2/registry"
	"keyayun.com/seal-micro-runner/pkg/config"
)

var (
	conf = config.Config
)

type Regs interface {
	Name() string
	GetReg() registry.Registry
}

func NewReg(name string) Regs {
	switch name {
	case "etcd":
		return newEtcdReg()
	case "consul":
		return newConsulReg()
	}
	return nil
}
