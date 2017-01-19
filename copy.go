package main

type Copy struct {
	PackageObject
	PackageObjectData

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
