package testsmocks

import (
	"github.com/spf13/afero"
	"github.com/stretchr/testify/mock"
)

type MtdUtilsMock struct {
	*mock.Mock
}

func (mum MtdUtilsMock) GetTargetDeviceFromMtdName(fsBackend afero.Fs, mtdname string) (string, error) {
	args := mum.Called(fsBackend, mtdname)
	return args.String(0), args.Error(1)
}

func (mum MtdUtilsMock) MtdIsNAND(devicepath string) (bool, error) {
	args := mum.Called(devicepath)
	return args.Bool(0), args.Error(1)
}
