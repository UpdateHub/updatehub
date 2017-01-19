package utils

// FIXME: test this package

import (
	"os/exec"
)

type CmdLine interface {
	Execute(cmdline string) ([]byte, error)
}

type CmdLineImpl struct {
}

func (cli *CmdLineImpl) Execute(cmdline string) ([]byte, error) {
	cmd := exec.Command(cmdline)
	return cmd.CombinedOutput()
}
