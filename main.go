package main

import (
	"os"

	"keyayun.com/seal-micro-runner/pkg/cmd"
	"keyayun.com/seal-micro-runner/pkg/logger"
)

var log = logger.WithNamespace("main")

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		if err != cmd.ErrUsage {
			log.Errorf("Error: %s ", err.Error())
			os.Exit(1)
		}
	}
}
