package metadata

import "bitbucket.org/ossystems/agent/handlers"

type ObjectData struct {
	Sha256sum string `json:"sha256sum"`
	Mode      string `json:"mode"`
}

func (o ObjectData) Setup() error {
	return nil
}

func (o ObjectData) Install() error {
	return nil
}

func (o ObjectData) Cleanup() error {
	return nil
}

func (o ObjectData) CheckRequirements() error {
	return nil
}

type Object interface {
	handlers.InstallUpdateHandler
}
