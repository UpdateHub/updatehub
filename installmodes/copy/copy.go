package copy

import (
	"bitbucket.org/ossystems/agent/installmodes"
	"bitbucket.org/ossystems/agent/pkg"
)

func init() {
	installmodes.RegisterInstallMode("copy", installmodes.InstallMode{
		CheckRequirements: checkRequirements,
		Instantiate:       instantiate,
	})
}

func checkRequirements() error {
	return nil
}

func instantiate() interface{} {
	return &CopyObject{}
}

type CopyObject struct {
	pkg.Object
	pkg.ObjectData

	TargetDevice string `json:"target-device"`
	TargetPath   string `json:"target-path,omitempty"`
}

func (cp CopyObject) CheckRequirements() error {
	return nil
}

func (cp CopyObject) Setup() error {
	return nil
}

func (cp CopyObject) Install() error {
	return nil
}

func (cp CopyObject) Cleanup() error {
	return nil
}
