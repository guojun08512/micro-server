package services

const DefaultVersion = "v0.1.0"
const ServiceRepository = "git@git.keyayun.com:keyayun/seal-micro-runner.git"

type serviceName = string

const (
	CarsCa     serviceName = "carsca"
	Cars                   = "cars"
	CarsUpdate             = "update"
	CarsPush               = "push"
	CarsRender             = "render"
)

type responseStatus = string

const (
	Success responseStatus = "Success"
)
