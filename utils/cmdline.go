package utils

import (
	"fmt"
	"os/exec"
	"strings"
)

type CmdLineExecuter interface {
	Execute(cmdline string) ([]byte, error)
}

type CmdLine struct {
}

func (cli *CmdLine) Execute(cmdline string) ([]byte, error) {
	list := strings.Split(cmdline, " ")
	cmd := exec.Command(list[0], list[1:]...)
	ret, err := cmd.CombinedOutput()

	if exitErr, ok := err.(*exec.ExitError); ok {
		if !exitErr.Success() {
			return ret, fmt.Errorf(fmt.Sprintf("Error executing command '%s': %s", cmdline, string(ret)))
		}
	}

	return ret, err
}
