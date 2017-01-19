package main

type Copy struct {
	Object

	TargetDevice string `json:"target-device"`
	TargetPath   string `json:"target-path,omitempty"`
}

func (cp Copy) CheckRequirements() error {
	return nil
}

func (cp Copy) Setup() error {
	return nil
}
