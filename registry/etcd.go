package registry

import (
	"github.com/micro/go-micro/v2/registry"
	"github.com/micro/go-micro/v2/registry/etcd"
)

type EtcdReg struct{}

func newEtcdReg() Regs {
	return &EtcdReg{}
}

func (e *EtcdReg) Name() string {
	return "etcd"
}

func (e *EtcdReg) GetReg() registry.Registry {
	return etcd.NewRegistry(func(opts *registry.Options) {
		opts.Addrs = conf.GetStringSlice("etcd.addrs")
	})
}
