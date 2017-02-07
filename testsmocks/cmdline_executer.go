package testsmocks

import "github.com/stretchr/testify/mock"

type CmdLineExecuterMock struct {
	*mock.Mock
}

func (clm CmdLineExecuterMock) Execute(cmdline string) ([]byte, error) {
	args := clm.Called(cmdline)
	return args.Get(0).([]byte), args.Error(1)
}
