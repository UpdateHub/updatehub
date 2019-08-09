/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package utils

import (
	"path"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestDataSha256sum(t *testing.T) {
	sha256sum := DataSha256sum([]byte("a"))
	assert.Equal(t, "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb", sha256sum)

	sha256sum = DataSha256sum([]byte("qwerty"))
	assert.Equal(t, "65e84be33532fb784c48129675f9eff3a682b27168c0ea744b2cf58ee02337c5", sha256sum)
}

func TestDataSha256sumWithUpdateMetadata(t *testing.T) {
	withActiveInactiveAndMultipleObjects := `{
  "product-uid": "123",
  "objects": [
    [
      { "mode": "test", "target": "/dev/xxa1", "sha256sum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" },
      { "mode": "test", "target": "/dev/xxa2", "sha256sum": "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb" }
    ]
    ,
    [
      { "mode": "test", "target": "/dev/xxb1", "sha256sum": "3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d" },
      { "mode": "test", "target": "/dev/xxb2", "sha256sum": "2e7d2c03a9507ae265ecf5b5356885a53393a2029d241394997265a1a25aefc6" }
    ]
  ]
}`

	sha256sum := DataSha256sum([]byte(withActiveInactiveAndMultipleObjects))
	assert.Equal(t, "2b01e5238bf2a5ce673f7f507b7d7916b51530c7229088ff4c30cdcccb5a92c9", sha256sum)
}

func TestFileSha256sum(t *testing.T) {
	fs := afero.NewMemMapFs()

	testPath, err := afero.TempDir(fs, "", "sha-test")
	assert.NoError(t, err)
	defer fs.RemoveAll(testPath)

	file1path := path.Join(testPath, "file1.txt")
	err = afero.WriteFile(fs, file1path, []byte("a"), 0666)
	assert.NoError(t, err)

	sha256sum, err := FileSha256sum(fs, file1path)
	assert.NoError(t, err)
	assert.Equal(t, "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb", sha256sum)

	file2path := path.Join(testPath, "file2.txt")
	err = afero.WriteFile(fs, file2path, []byte("qwerty"), 0666)
	assert.NoError(t, err)

	sha256sum, err = FileSha256sum(fs, file2path)
	assert.NoError(t, err)
	assert.Equal(t, "65e84be33532fb784c48129675f9eff3a682b27168c0ea744b2cf58ee02337c5", sha256sum)
}

func TestFileSha256sumWithReadFileError(t *testing.T) {
	fs := afero.NewMemMapFs()

	sha256sum, err := FileSha256sum(fs, "/tmp/inexistant")
	assert.EqualError(t, err, "open /tmp/inexistant: file does not exist")
	assert.Equal(t, "", sha256sum)
}
