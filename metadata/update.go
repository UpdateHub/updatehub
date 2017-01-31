package metadata

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
)

type UpdateMetadata struct {
	ProductUID string     `json:"product-uid"`
	Version    string     `json:"version"`
	Objects    [][]Object `json:"-"`
}

func (m UpdateMetadata) Checksum() (string, error) {
	var wrapper struct {
		UpdateMetadata
		Objects [][]Object `json:"objects"`
	}

	wrapper.UpdateMetadata = m
	wrapper.Objects = m.Objects

	data, err := json.Marshal(wrapper)
	if err != nil {
		return "", err
	}

	r := bytes.NewReader(data)

	hash := sha256.New()
	_, err = io.Copy(hash, r)
	if err != nil {
		return "", nil
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
