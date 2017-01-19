package main

type Object struct {
	InstallUpdateHandler

	Sha256sum string `json:"sha256sum"`
	Mode      string `json:"mode"`
}

func (o Object) Setup() error {
	return nil
}

func (o Object) CheckRequirements() error {
	return nil
}
