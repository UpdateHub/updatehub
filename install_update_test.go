package main

import (
	"errors"
	"testing"

	"bitbucket.org/ossystems/agent/metadata"

	"github.com/stretchr/testify/mock"
)

type FakeObject struct {
	mock.Mock
	metadata.Object
}

func (f *FakeObject) CheckRequirements() error {
	f.Called()
	return nil
}

func (f *FakeObject) Setup() error {
	f.Called()
	return nil
}

func (f *FakeObject) Install() error {
	f.Called()
	return nil
}

func (f *FakeObject) Cleanup() error {
	f.Called()
	return nil
}

func TestInstallUpdate(t *testing.T) {
	f := &FakeObject{}

	f.On("CheckRequirements").Return(errors.New(""))
	f.On("Setup").Return()
	f.On("Install").Return()
	f.On("Cleanup").Return()

	InstallUpdate(f)

	f.AssertExpectations(t)
}
