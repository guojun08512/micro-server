package cmd

import (
	"errors"

	"github.com/spf13/cobra"
	"keyayun.com/seal-micro-runner/pkg/config"
	"keyayun.com/seal-micro-runner/pkg/logger"
	_ "keyayun.com/seal-micro-runner/pkg/redis"
)

var (
	conf = config.Config
	// ErrUsage is returned by the cmd.Usage() method
	ErrUsage = errors.New("Bad usage of command")
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "runner-server",
	Short: "runner server is run alg service",
	Long: `A Fast and Flexible Static Site Generator built with
				  love by spf13 and friends in Go.
				  Complete documentation is available at http://hugo.spf13.com`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	},
	SilenceUsage: true,
}

func init() {
	logger.Init(logger.NewOptions(
		conf.GetString("log.level"),
		conf.GetBool("log.report_caller"),
		conf.GetBool("log.os_out"),
		conf.GetStringMap("log.formatter"),
		conf.GetStringMap("log.output")))
	usageFunc := RootCmd.UsageFunc()
	RootCmd.SetUsageFunc(func(cmd *cobra.Command) error {
		usageFunc(cmd)
		return ErrUsage
	})
}
