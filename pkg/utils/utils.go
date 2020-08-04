package utils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"keyayun.com/seal-micro-runner/pkg/logger"
)

var (
	lgr = logger.WithNamespace("utils")
)

// DirExists returns whether or not the directory exists on the current file
// system.
func DirExists(name string) (bool, error) {
	infos, err := os.Stat(name)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !infos.IsDir() {
		return false, fmt.Errorf("Path %s is not a directory", name)
	}
	return true, nil
}

// Execute 执行命令行
func Execute(name, command string) (*bytes.Buffer, error) {
	lgr.Infof("Execute `%s` command: \n%s", name, command)
	cmd := exec.Command("bash", "-c", command)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	lgr.Infof("Execute `%s` command output: \n%s", name, out.String())
	if stderr.Len() > 0 {
		lgr.Errorf("Execute `%s` command stderr: \n%s", name, stderr.String())
		return nil, fmt.Errorf(stderr.String())
	}
	if err != nil {
		lgr.Errorf("Execute `%s` command failed as %v: \n%s", name, err, stderr.String())
		return nil, err
	}
	return &out, nil
}
