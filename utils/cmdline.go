package utils

import (
	"fmt"
	"os/exec"

	shellwords "github.com/mattn/go-shellwords"
)

type CmdLineExecuter interface {
	Execute(cmdline string) ([]byte, error)
}

type CmdLine struct {
}

func (cl *CmdLine) Execute(cmdline string) ([]byte, error) {
	p := shellwords.NewParser()
	list, err := p.Parse(cmdline)

	cmd := exec.Command(list[0], list[1:]...)
	ret, err := cmd.CombinedOutput()

	if exitErr, ok := err.(*exec.ExitError); ok {
		if !exitErr.Success() {
			return ret, fmt.Errorf(fmt.Sprintf("Error executing command '%s': %s", cmdline, string(ret)))
		}
	}

	return ret, err
}
