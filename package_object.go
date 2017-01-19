package main

import "encoding/json"

type PackageObjectData struct {
	Sha256sum string `json:"sha256sum"`
	Mode      string `json:"mode"`
}

func (o PackageObjectData) Setup() error {
	return nil
}

func (o PackageObjectData) Install() error {
	return nil
}

func (o PackageObjectData) Cleanup() error {
	return nil
}

func (o PackageObjectData) CheckRequirements() error {
	return nil
}

type PackageObject interface {
	InstallUpdateHandler
}

func PackageObjectFromJSON(bytes []byte) (PackageObject, error) {
	var v interface{}

	err := json.Unmarshal(bytes, &v)
	if err != nil {
		return nil, err
	}

	var obj PackageObject

	switch v.(map[string]interface{})["mode"] {
	case "copy":
		obj = &Copy{}
	}

	json.Unmarshal(bytes, &obj)

	return obj, nil
}
