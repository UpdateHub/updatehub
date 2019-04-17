package server

import (
	"archive/zip"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/UpdateHub/updatehub/metadata"
	"github.com/stretchr/testify/assert"
)

func TestNewUpdatePackage(t *testing.T) {
	bytes, err := json.Marshal(metadata.UpdateMetadata{
		ProductUID: "548873c4d4e8e751fdd46c38a3d5a8656cf87bf27a404f346ad58086f627a4ea",
	})
	assert.NoError(t, err)

	f, err := generateUpdatePackage(string(bytes))
	assert.NoError(t, err)

	defer os.RemoveAll(f.Name())

	pkg, err := NewUpdatePackage(f)
	assert.NotNil(t, pkg)
	assert.NoError(t, err)
}

func TestFetchUpdatePackage(t *testing.T) {
	bytes, err := json.Marshal(metadata.UpdateMetadata{
		ProductUID: "548873c4d4e8e751fdd46c38a3d5a8656cf87bf27a404f346ad58086f627a4ea",
	})
	assert.NoError(t, err)

	pkg, err := generateUpdatePackage(string(bytes))
	assert.NoError(t, err)

	defer os.RemoveAll(pkg.Name())

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, pkg.Name())
	}))
	defer ts.Close()

	uri, _ := url.Parse(ts.URL)
	pkg2, err := fetchUpdatePackage(uri)
	assert.NoError(t, err)
	defer os.RemoveAll(pkg2.file.Name())
}

func TestParseUpdatePackage(t *testing.T) {
	bytes, err := json.Marshal(metadata.UpdateMetadata{
		ProductUID: "548873c4d4e8e751fdd46c38a3d5a8656cf87bf27a404f346ad58086f627a4ea",
	})
	assert.NoError(t, err)

	pkg, err := generateUpdatePackage(string(bytes))
	assert.NoError(t, err)

	defer os.RemoveAll(pkg.Name())

	updateMetadata, _, err := parseUpdatePackage(pkg)
	assert.NoError(t, err)
	assert.Equal(t, bytes, updateMetadata)
}

func generateUpdatePackage(metadata string) (*os.File, error) {
	file, err := ioutil.TempFile("", "uhupkg")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	zw := zip.NewWriter(file)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	sha256sum := sha256.Sum256([]byte(metadata))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sha256sum[:])
	if err != nil {
		return nil, err
	}

	var files = []struct {
		name, content string
	}{
		{"metadata", metadata},
		{"signature", string(signature)},
	}

	for _, file := range files {
		f, err := zw.Create(file.name)
		if err != nil {
			return nil, err
		}

		if _, err = f.Write([]byte(file.content)); err != nil {
			return nil, err
		}
	}

	if err = zw.Close(); err != nil {
		return nil, err
	}

	return file, nil
}
