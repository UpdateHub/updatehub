/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package libarchive

import (
	"archive/tar"
	"compress/gzip"
	"log"
	"os"
	"path"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestUnpackIntegration(t *testing.T) {
	memFs := afero.NewOsFs()

	targetPath, err := afero.TempDir(memFs, "", "Unpack-test")
	assert.NoError(t, err)
	defer memFs.RemoveAll(targetPath)

	tarballPath, err := generateTarball(memFs)
	assert.NoError(t, err)

	originalDir, err := os.Getwd()
	assert.NoError(t, err)

	la := LibArchive{}
	err = la.Unpack(tarballPath, targetPath, false)
	assert.NoError(t, err)

	files, err := afero.ReadDir(memFs, targetPath)
	assert.NoError(t, err)
	retrievedFiles := []string{}
	for _, file := range files {
		retrievedFiles = append(retrievedFiles, file.Name())
	}
	expectedFiles := []string{"source1.txt", "source2.txt", "source3.txt"}
	assert.Equal(t, expectedFiles, retrievedFiles)

	data1, err := afero.ReadFile(memFs, path.Join(targetPath, "source1.txt"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("content1"), data1)

	data2, err := afero.ReadFile(memFs, path.Join(targetPath, "source2.txt"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("content2"), data2)

	data3, err := afero.ReadFile(memFs, path.Join(targetPath, "source3.txt"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("content3"), data3)

	newDir, err := os.Getwd()
	assert.NoError(t, err)
	assert.Equal(t, originalDir, newDir)
}

func TestUnpackWithExtractTarballError(t *testing.T) {
	fs := afero.NewOsFs()

	targetPath, err := afero.TempDir(fs, "", "Unpack-test")
	assert.NoError(t, err)
	defer fs.RemoveAll(targetPath)

	tarballPath, err := generateCorruptedTarball(fs)
	assert.NoError(t, err)

	originalDir, err := os.Getwd()
	assert.NoError(t, err)

	la := LibArchive{}
	err = la.Unpack(tarballPath, targetPath, false)
	assert.EqualError(t, err, "Unrecognized archive format")

	newDir, err := os.Getwd()
	assert.NoError(t, err)
	assert.Equal(t, originalDir, newDir)
}

func generateCorruptedTarball(fsBackend afero.Fs) (string, error) {
	tarballPath := "/tmp/output.tar.gz"
	file, err := fsBackend.Create(tarballPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = file.Write([]byte("random stuff"))
	if err != nil {
		return "", err
	}

	return tarballPath, nil
}

func generateTarball(fsBackend afero.Fs) (string, error) {
	tarballPath := "/tmp/output.tar.gz"
	file, err := fsBackend.Create(tarballPath)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	var files = []struct {
		Name, Body string
	}{
		{"source1.txt", "content1"},
		{"source2.txt", "content2"},
		{"source3.txt", "content3"},
	}

	for _, file := range files {
		hdr := &tar.Header{
			Name: file.Name,
			Mode: 0600,
			Size: int64(len(file.Body)),
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return "", err
		}

		if _, err := tw.Write([]byte(file.Body)); err != nil {
			return "", err
		}
	}

	return tarballPath, nil
}
