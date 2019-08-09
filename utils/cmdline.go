/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

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
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(list[0], list[1:]...)
	ret, err := cmd.CombinedOutput()

	if exitErr, ok := err.(*exec.ExitError); ok {
		if !exitErr.Success() {
			return ret, fmt.Errorf("Error executing command '%s': %s", cmdline, string(ret))
		}
	}

	return ret, err
}
