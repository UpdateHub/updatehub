package copy

import "bitbucket.org/ossystems/agent/pkg"
import "bitbucket.org/ossystems/agent/plugins"

func init() {
	plugins.RegisterPlugin("copy", plugins.Plugin{
		CheckRequirements: checkRequirements,
		Instantiate:       instantiate,
	})
}

func checkRequirements() error {
	return nil
}

func instantiate() interface{} {
	return &Copy{}
}

type Copy struct {
	pkg.Object
	pkg.ObjectData

	TargetDevice string `json:"target-device"`
	TargetPath   string `json:"target-path,omitempty"`
}

func (cp Copy) CheckRequirements() error {
	return nil
}

func (cp Copy) Setup() error {
	return nil
}

func (cp Copy) Install() error {
	return nil
}

func (cp Copy) Cleanup() error {
	return nil
}
