/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

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

func (m *UpdateMetadata) Checksum() (string, error) {
	var wrapper struct {
		UpdateMetadata
		Objects [][]Object `json:"objects"`
	}

	wrapper.UpdateMetadata = *m
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
