package metadata

import "bitbucket.org/ossystems/agent/handlers"

// ObjectMetadata contains the common properties of a package's object from JSON metadata
type ObjectMetadata struct {
	Object `json:"-"`

	Sha256sum  string `json:"sha256sum"`
	Mode       string `json:"mode"`
	Compressed bool   `json:"bool"`
}

func (o ObjectMetadata) GetObjectMetadata() ObjectMetadata {
	return o
}

type CompressedObject struct {
	CompressedSize   float64 `json:"required-compressed-size"`
	UncompressedSize float64 `json:"required-uncompressed-size"`
}

type Object interface {
	handlers.InstallUpdateHandler

	GetObjectMetadata() ObjectMetadata
}
