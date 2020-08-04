package cmd

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/micro/go-micro/v2"
	"github.com/micro/go-micro/v2/server"
	"github.com/spf13/cobra"

	"keyayun.com/seal-micro-runner/pkg/logger"
	"keyayun.com/seal-micro-runner/pkg/services"
	"keyayun.com/seal-micro-runner/registry"

	pb "keyayun.com/seal-micro-runner/pkg/proto"
	"keyayun.com/seal-micro-runner/pkg/services/carsrender"
)

var (
	log = logger.WithNamespace("cars-cmd")
)

func createService(name string) micro.Service {
	serviceName := fmt.Sprintf("%s.%s", conf.GetString("task.prefix"), name)
	reg := registry.NewReg("consul")
	return micro.NewService(
		micro.RegisterTTL(time.Second*30),
		micro.RegisterInterval(time.Second*30),
		micro.Name(serviceName),
		micro.Registry(reg.GetReg()),
	)
}

func carsRenderServiceStartUp() error {
	serv := createService(services.CarsRender)
	serverID := uuid.New().String()
	serv.Server().Init(server.Id(serverID))
	// Register Handlers
	sHandler := carsrender.NewCarsRenderService()
	sHandler.InitService(serverID)
	err := pb.RegisterServicesHandler(serv.Server(), sHandler)
	if err != nil {
		log.Error(err)
		return err
	}
	// Run server
	if err := serv.Run(); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

var carsRenderCmd = &cobra.Command{
	Use:   "render",
	Short: "micro-server cars-service render",
	RunE: func(cmd *cobra.Command, args []string) error {
		return carsRenderServiceStartUp()
	},
}

var servicesCarsGroup = &cobra.Command{
	Use:   "cars-service",
	Short: "micro-server cars-service",
	RunE: func(cmd *cobra.Command, Args []string) error {
		return cmd.Usage()
	},
}

func init() {
	servicesCarsGroup.AddCommand(carsRenderCmd)
	RootCmd.AddCommand(servicesCarsGroup)
}
