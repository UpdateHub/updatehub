package main

import "fmt"

type Copy struct {
	PackageObject
	PackageObjectData

	TargetDevice string `json:"target-device"`
	TargetPath   string `json:"target-path,omitempty"`
}

func (cp Copy) CheckRequirements() error {
	fmt.Println("do copy")
	return nil
}

func (cp Copy) Setup() error {
	return nil
}
