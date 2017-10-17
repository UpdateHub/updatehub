/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package metadata

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"

	"github.com/updatehub/updatehub/utils"
)

const UpdateMetadataFilename = "updatemetadata.json"

type UpdateMetadata struct {
	ProductUID        string      `json:"product-uid"`
	Version           string      `json:"version"`
	Objects           [][]Object  `json:"-"`
	SupportedHardware interface{} `json:"supported-hardware"`
	RawBytes          []byte
}

func NewUpdateMetadata(bytes []byte) (*UpdateMetadata, error) {
	var wrapper struct {
		UpdateMetadata
		RawObjects [][]interface{} `json:"objects"`
	}

	err := json.Unmarshal(bytes, &wrapper)
	if err != nil {
		return nil, err
	}

	// Unwraps metadata
	metadata := wrapper.UpdateMetadata
	metadata.RawBytes = bytes

	for _, list := range wrapper.RawObjects {
		var objects []Object

		for _, obj := range list {
			// It is safe to ignore errors here
			b, _ := json.Marshal(obj)

			o, err := NewObjectMetadata(b)
			if err != nil {
				return nil, err
			}

			objects = append(objects, o)
		}

		metadata.Objects = append(metadata.Objects, objects)
	}

	return &metadata, nil
}

func (m *UpdateMetadata) PackageUID() string {
	return utils.DataSha256sum(m.RawBytes)
}

func (m *UpdateMetadata) VerifySignature(key *rsa.PublicKey, signature []byte) bool {
	sha256sum := sha256.Sum256(m.RawBytes)
	err := rsa.VerifyPKCS1v15(key, crypto.SHA256, sha256sum[:], signature)
	return err == nil
}
