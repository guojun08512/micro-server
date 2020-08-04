package registry

import (
	"github.com/micro/go-micro/v2/registry"
	"github.com/micro/go-plugins/registry/consul/v2"
)

type ConsulReg struct{}

func newConsulReg() Regs {
	return &ConsulReg{}
}

func (conReg *ConsulReg) Name() string {
	return "consul"
}

func (conReg *ConsulReg) GetReg() registry.Registry {
	return consul.NewRegistry(func(opts *registry.Options) {
		opts.Addrs = conf.GetStringSlice("consul.addrs")
	})
}
