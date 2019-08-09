/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"

	"github.com/spf13/afero"
)

func DataSha256sum(data []byte) string {
	hash := sha256.New()

	// hash.Hash "Write()" never returns an error
	_, _ = hash.Write(data)

	return hex.EncodeToString(hash.Sum(nil))
}

func FileSha256sum(fsb afero.Fs, filepath string) (string, error) {
	f, err := fsb.OpenFile(filepath, os.O_RDONLY, 0)
	if err != nil {
		return "", err
	}
	defer f.Close()

	return FsbFileSha256sum(f), nil
}

func FsbFileSha256sum(f afero.File) string {
	hash := sha256.New()

	_, _ = io.Copy(hash, f)

	return hex.EncodeToString(hash.Sum(nil))

}
