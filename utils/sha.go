/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package utils

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/spf13/afero"
)

func DataSha256sum(data []byte) string {
	hash := sha256.New()

	// hash.Hash "Write()" never returns an error
	_, _ = hash.Write(data)

	return hex.EncodeToString(hash.Sum(nil))
}

func FileSha256sum(fsb afero.Fs, filepath string) (string, error) {
	data, err := afero.ReadFile(fsb, filepath)
	if err != nil {
		return "", err
	}

	return DataSha256sum(data), nil
}
