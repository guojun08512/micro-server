#
#
#

SHELL = /bin/sh
UNAME := $(shell uname)
protoc = /usr/local/bin/protoc
GOPATH = /Users/curacloud/go

grpc:
	$(protoc) --plugin=protoc-gen-go=$(GOPATH)/bin/protoc-gen-go --plugin=protoc-gen-micro=$(GOPATH)/bin/protoc-gen-micro --proto_path=$(GOPATH)/src:. --micro_out=. --go_out=. ./pkg/proto/service.proto