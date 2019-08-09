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
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCmdLineExecuteWithSuccess(t *testing.T) {
	testCases := []struct {
		Name           string
		BinaryContent  string
		Args           []string
		ExpectedOutput []byte
	}{
		{
			"WithOutputOnStdoutOnly",
			`#!/bin/sh
echo "stdout string"
exit 0
`,
			[]string(nil),
			[]byte("stdout string\n"),
		},
		{
			"WithOutputOnStderrOnly",
			`#!/bin/sh
>&2 echo -n "error string"
exit 0
`,
			[]string(nil),
			[]byte("error string"),
		},
		{
			"WithOutputOnBothStdoutAndStderr",
			`#!/bin/sh
echo "stdout string"
>&2 echo -n "error string"
exit 0
`,
			[]string(nil),
			[]byte("stdout string\nerror string"),
		},
		{
			"WithMultipleArgs",
			`#!/bin/sh
echo "stdout string $@"
exit 0
`,
			[]string{"firstArg", "secondArg", "thirdArg"},
			[]byte("stdout string firstArg secondArg thirdArg\n"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			testPath, err := ioutil.TempDir("", "CmdLineExecute-test")
			assert.Nil(t, err)
			defer os.RemoveAll(testPath)

			fakeCmdPath := path.Join(testPath, "binary")
			fakeCmdFile, err := os.Create(fakeCmdPath)
			assert.Nil(t, err)
			err = os.Chmod(fakeCmdPath, 0777)
			assert.Nil(t, err)
			_, err = fakeCmdFile.WriteString(tc.BinaryContent)
			assert.Nil(t, err)
			err = fakeCmdFile.Close()
			assert.Nil(t, err)

			c := &CmdLine{}
			cmdString := fakeCmdPath + " " + strings.Join(tc.Args[:], " ")
			output, err := c.Execute(cmdString)

			assert.NoError(t, err)
			assert.Equal(t, tc.ExpectedOutput, output)
		})
	}
}

func TestCmdLineExecuteWithNestedDoubleQuotes(t *testing.T) {
	testPath, err := ioutil.TempDir("", "CmdLineExecute-test")
	assert.Nil(t, err)
	defer os.RemoveAll(testPath)

	binaryContent := `#!/bin/sh
echo "stdout string $@"
exit 0
`

	fakeCmdPath := path.Join(testPath, "binary")
	fakeCmdFile, err := os.Create(fakeCmdPath)
	assert.NoError(t, err)
	err = os.Chmod(fakeCmdPath, 0777)
	assert.NoError(t, err)
	_, err = fakeCmdFile.WriteString(binaryContent)
	assert.NoError(t, err)
	err = fakeCmdFile.Close()
	assert.NoError(t, err)

	outputPath := path.Join(testPath, "output.txt")

	c := &CmdLine{}
	cmdString := fmt.Sprintf("sh -c \"%s -c arg.gz > %s\"", fakeCmdPath, outputPath)
	output, err := c.Execute(cmdString)

	assert.NoError(t, err)
	assert.Equal(t, []byte(""), output)

	data, err := ioutil.ReadFile(outputPath)
	assert.NoError(t, err)
	assert.Equal(t, []byte("stdout string -c arg.gz\n"), data)
}

func TestCmdLineExecuteWithBinaryNotFound(t *testing.T) {
	testPath, err := ioutil.TempDir("", "CmdLineExecute-test")
	assert.Nil(t, err)
	defer os.RemoveAll(testPath)

	fakeCmdPath := path.Join(testPath, "inexistant")

	c := &CmdLine{}
	output, err := c.Execute(fakeCmdPath)

	assert.EqualError(t, err, fmt.Sprintf("fork/exec %s: no such file or directory", fakeCmdPath))
	assert.Equal(t, []byte(nil), output)
}

func TestCmdLineExecuteWithInvalidCmdLine(t *testing.T) {
	testPath, err := ioutil.TempDir("", "CmdLineExecute-test")
	assert.Nil(t, err)
	defer os.RemoveAll(testPath)

	c := &CmdLine{}
	output, err := c.Execute(`tee "%s`)

	assert.EqualError(t, err, "invalid command line string")
	assert.Equal(t, []byte(nil), output)
}

func TestCmdLineExecuteWithBinaryError(t *testing.T) {
	testCases := []struct {
		Name           string
		BinaryContent  string
		Args           []string
		ExpectedOutput []byte
	}{
		{
			"WithOutputOnStderrOnly",
			`#!/bin/sh
>&2 echo -n "error string"
exit 1
`,
			[]string(nil),
			[]byte("error string"),
		},
		{
			"WithOutputOnStdoutOnly",
			`#!/bin/sh
echo "stdout string"
exit 1
`,
			[]string(nil),
			[]byte("stdout string\n"),
		},
		{
			"WithOutputOnBothStderrAndStdout",
			`#!/bin/sh
echo "stdout string"
>&2 echo -n "error string"
exit 1
`,
			[]string(nil),
			[]byte("stdout string\nerror string"),
		},
		{
			"WithMultipleArgs",
			`#!/bin/sh
>&2 echo -n "error string $@"
exit 1
`,
			[]string{"firstArg", "secondArg", "thirdArg"},
			[]byte("error string firstArg secondArg thirdArg"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			testPath, err := ioutil.TempDir("", "CmdLineExecute-test")
			assert.Nil(t, err)
			defer os.RemoveAll(testPath)

			fakeCmdPath := path.Join(testPath, "binary-with-error")
			fakeCmdFile, err := os.Create(fakeCmdPath)
			assert.Nil(t, err)
			err = os.Chmod(fakeCmdPath, 0777)
			assert.Nil(t, err)
			_, err = fakeCmdFile.WriteString(tc.BinaryContent)
			assert.Nil(t, err)
			err = fakeCmdFile.Close()
			assert.Nil(t, err)

			c := &CmdLine{}
			cmdString := fakeCmdPath + " " + strings.Join(tc.Args[:], " ")
			output, err := c.Execute(cmdString)

			assert.EqualError(t, err, fmt.Sprintf("Error executing command '%s': %s", cmdString, tc.ExpectedOutput))
			assert.Equal(t, tc.ExpectedOutput, output)
		})
	}
}
