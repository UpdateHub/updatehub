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
