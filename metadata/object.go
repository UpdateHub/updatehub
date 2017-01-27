package metadata

import "bitbucket.org/ossystems/agent/handlers"

// ObjectData contains the common properties of a package's object from JSON metadata
type ObjectData struct {
	Object `json:"-"`

	Sha256sum  string `json:"sha256sum"`
	Mode       string `json:"mode"`
	Compressed bool   `json:"bool"`
}

func (o ObjectData) GetObjectData() ObjectData {
	return o
}

type CompressedObject struct {
	CompressedSize   float64 `json:"required-compressed-size"`
	UncompressedSize float64 `json:"required-uncompressed-size"`
}

type Object interface {
	handlers.InstallUpdateHandler

	GetObjectData() ObjectData
}
