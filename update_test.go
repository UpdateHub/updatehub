package main

import (
	"testing"

	"github.com/stretchr/testify/mock"
)

type FakeObject struct {
	mock.Mock
	PackageObject
}

func (f *FakeObject) CheckRequirements() error {
	f.Called()
	return nil
}

func (f *FakeObject) Setup() error {
	f.Called()
	return nil
}

func TestInstallUpdate(t *testing.T) {
	f := &FakeObject{}

	f.On("CheckRequirements").Return()
	f.On("Setup").Return()

	InstallUpdate(f)

	f.AssertExpectations(t)
}
