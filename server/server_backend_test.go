/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package server

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/UpdateHub/updatehub/libarchive"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/libarchivemock"
	"github.com/UpdateHub/updatehub/utils"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	ValidJSONMetadata = `{
          "product-uid": "548873c4d4e8e751fdd46c38a3d5a8656cf87bf27a404f346ad58086f627a4ea",
          "supported-hardware": [
            "hardware1-revA",
            "hardware2-revB"
          ],
          "objects": [
            [
              { "mode": "test", "sha256sum": "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa" }
            ]
          ]
        }`

	InvalidJSONMetadata = `{
          "product-
        }`
)

func TestNewServerBackendWithNonExistantDirectoryError(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	dirPath := path.Join(testPath, "inexistant-dir")

	sb, err := NewServerBackend(lam, dirPath)

	assert.EqualError(t, err, fmt.Sprintf("stat %s: no such file or directory", dirPath))
	assert.Nil(t, sb)

	lam.AssertExpectations(t)
}

func TestNewServerBackendWithNotADirectoryError(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, metadata.UpdateMetadataFilename)

	err = ioutil.WriteFile(updateMetadataFilePath, []byte(ValidJSONMetadata), 0666)
	assert.NoError(t, err)

	sb, err := NewServerBackend(lam, updateMetadataFilePath)

	assert.EqualError(t, err, fmt.Sprintf("%s: not a directory", updateMetadataFilePath))
	assert.Nil(t, sb)

	lam.AssertExpectations(t)
}

func TestNewServerBackend(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, metadata.UpdateMetadataFilename)

	err = ioutil.WriteFile(updateMetadataFilePath, []byte(ValidJSONMetadata), 0666)
	assert.NoError(t, err)

	sb, err := NewServerBackend(lam, testPath)
	assert.NoError(t, err)

	routes := sb.Routes()

	assert.Equal(t, 3, len(routes))

	assert.Equal(t, "POST", routes[0].Method)
	assert.Equal(t, "/upgrades", routes[0].Path)
	assert.Equal(t, reflect.ValueOf(sb.getUpdateMetadata).Pointer(), reflect.ValueOf(routes[0].Handle).Pointer())

	assert.Equal(t, "POST", routes[1].Method)
	assert.Equal(t, "/report", routes[1].Path)
	assert.Equal(t, reflect.ValueOf(sb.reportStatus).Pointer(), reflect.ValueOf(routes[1].Handle).Pointer())

	assert.Equal(t, "GET", routes[2].Method)
	assert.Equal(t, "/products/:product/packages/:package/objects/:object", routes[2].Path)
	assert.Equal(t, reflect.ValueOf(sb.getObject).Pointer(), reflect.ValueOf(routes[2].Handle).Pointer())

	lam.AssertExpectations(t)
}

func TestParseUpdateMetadataWithStatError(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, metadata.UpdateMetadataFilename)

	err = ioutil.WriteFile(updateMetadataFilePath, []byte(ValidJSONMetadata), 0666)
	assert.NoError(t, err)

	sb, err := NewServerBackend(lam, testPath)
	assert.NoError(t, err)

	os.Remove(updateMetadataFilePath)

	data, err := sb.parseUpdateMetadata()
	assert.EqualError(t, err, fmt.Sprintf("stat %s: no such file or directory", updateMetadataFilePath))
	assert.Nil(t, data)

	lam.AssertExpectations(t)
}

func TestParseUpdateMetadataWithUnmarshalError(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, metadata.UpdateMetadataFilename)

	err = ioutil.WriteFile(updateMetadataFilePath, []byte(InvalidJSONMetadata), 0666)
	assert.NoError(t, err)

	sb, err := NewServerBackend(lam, testPath)
	assert.NoError(t, err)

	data, err := sb.parseUpdateMetadata()
	assert.EqualError(t, err, fmt.Sprintf("Invalid update metadata: invalid character '\\n' in string literal"))
	assert.Nil(t, data)

	lam.AssertExpectations(t)
}

func TestParseUhuPkgWithNewReaderError(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	pkgpath := path.Join(testPath, "dummy")

	a := libarchive.LibArchive{}.NewRead()
	lam.On("NewRead").Return(a)
	lam.On("ReadSupportFilterAll", a).Return(fmt.Errorf("libarchive error"))
	lam.On("ReadFree", a)

	sb, err := NewServerBackend(lam, testPath)
	assert.NoError(t, err)

	data, err := sb.parseUhuPkg(pkgpath)
	assert.EqualError(t, err, "libarchive error")

	assert.Nil(t, data)
	assert.Nil(t, sb.selectedPackage)

	lam.AssertExpectations(t)
}

func TestParseUhuPkgWithExtractFileError(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	pkgpath := path.Join(testPath, "dummy")

	a := libarchive.LibArchive{}.NewRead()
	lam.On("NewRead").Return(a)
	lam.On("ReadSupportFilterAll", a).Return(nil)
	lam.On("ReadSupportFormatRaw", a).Return(nil)
	lam.On("ReadSupportFormatEmpty", a).Return(nil)
	lam.On("ReadSupportFormatAll", a).Return(nil)
	lam.On("ReadOpenFileName", a, pkgpath, 10240).Return(nil)
	lam.On("ReadNextHeader", a, mock.Anything).Return(fmt.Errorf("libarchive extract error"))

	sb, err := NewServerBackend(lam, testPath)
	assert.NoError(t, err)

	data, err := sb.parseUhuPkg(pkgpath)
	assert.EqualError(t, err, fmt.Sprintf("file 'metadata' not found in: '%s'", pkgpath))

	assert.Nil(t, data)
	assert.Nil(t, sb.selectedPackage)

	lam.AssertExpectations(t)
}

func TestProcessDirectoryWithMoreThanOnePackage(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	pkg1 := path.Join(testPath, "pkg1.uhupkg")
	err = ioutil.WriteFile(pkg1, []byte("dummy_content"), 0666)
	assert.NoError(t, err)

	pkg2 := path.Join(testPath, "pkg2.uhupkg")
	err = ioutil.WriteFile(pkg2, []byte("dummy_content"), 0666)
	assert.NoError(t, err)

	sb, err := NewServerBackend(lam, testPath)
	assert.NoError(t, err)

	err = sb.ProcessDirectory()
	assert.Equal(t, fmt.Errorf("the path provided must not have more than 1 package. Found: 2"), err)

	lam.AssertExpectations(t)
}

func TestProcessDirectoryWithExactlyOneUpdateMetadata(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, metadata.UpdateMetadataFilename)

	err = ioutil.WriteFile(updateMetadataFilePath, []byte(ValidJSONMetadata), 0666)
	assert.NoError(t, err)

	sb, err := NewServerBackend(lam, testPath)
	assert.NoError(t, err)

	err = sb.ProcessDirectory()
	assert.NoError(t, err)
	assert.Equal(t, "", sb.selectedPackage.uhupkgPath)
	assert.Equal(t, ValidJSONMetadata, string(sb.selectedPackage.updateMetadata))

	lam.AssertExpectations(t)
}

func TestProcessDirectoryWithExactlyOneUhuPkg(t *testing.T) {
	memFs := afero.NewOsFs()

	la := &libarchive.LibArchive{}

	tarballPath, err := generateUhupkg(memFs, true)
	assert.NoError(t, err)

	testPath, err := afero.TempDir(memFs, "", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	pkgpath := path.Join(testPath, "name.uhupkg")

	os.Link(tarballPath, pkgpath)

	sb, err := NewServerBackend(la, testPath)
	assert.NoError(t, err)

	err = sb.ProcessDirectory()
	assert.NoError(t, err)

	assert.NotNil(t, sb.selectedPackage)
	assert.Equal(t, pkgpath, sb.selectedPackage.uhupkgPath)
	assert.Equal(t, ValidJSONMetadata, string(sb.selectedPackage.updateMetadata))
}

func TestUpgradesRoute(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	// setup filesystem
	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, metadata.UpdateMetadataFilename)

	err = ioutil.WriteFile(updateMetadataFilePath, []byte(ValidJSONMetadata), 0666)
	assert.NoError(t, err)

	// setup server
	sb, err := NewServerBackend(lam, testPath)
	assert.NoError(t, err)

	router := NewBackendRouter(sb)
	server := httptest.NewServer(router.HTTPRouter)

	err = sb.ProcessDirectory()
	assert.NoError(t, err)

	// do the request
	r, err := http.Post(server.URL+"/upgrades", "application/json", bytes.NewBuffer([]byte("{\"content\": true}")))
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, r.StatusCode)
	assert.Equal(t, []byte(ValidJSONMetadata), bodyContent)

	lam.AssertExpectations(t)
}

func TestUpgradesRouteWithMetadataNotFound(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	// setup filesystem
	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	// setup server
	sb, err := NewServerBackend(lam, testPath)
	assert.NoError(t, err)

	router := NewBackendRouter(sb)
	server := httptest.NewServer(router.HTTPRouter)

	// do the request
	r, err := http.Post(server.URL+"/upgrades", "application/json", bytes.NewBuffer([]byte("{\"content\": true}")))
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, r.StatusCode)
	assert.Equal(t, []byte("404 page not found\n"), bodyContent)

	lam.AssertExpectations(t)
}

func TestGetObjectRoute(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	productUID := "a"
	packageUID := "b"
	object := "c"

	// setup filesystem
	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, metadata.UpdateMetadataFilename)

	err = ioutil.WriteFile(updateMetadataFilePath, []byte(ValidJSONMetadata), 0666)
	assert.NoError(t, err)

	pkgdir := path.Join(testPath, productUID, packageUID)
	err = os.MkdirAll(pkgdir, 0777)
	assert.NoError(t, err)

	objdir := path.Join(pkgdir, object)
	err = ioutil.WriteFile(objdir, []byte("object_content"), 0777)
	assert.NoError(t, err)

	// setup server
	sb, err := NewServerBackend(lam, testPath)
	assert.NoError(t, err)

	router := NewBackendRouter(sb)
	server := httptest.NewServer(router.HTTPRouter)

	err = sb.ProcessDirectory()
	assert.NoError(t, err)

	// do the request
	finalURL := fmt.Sprintf("%s/products/%s/packages/%s/objects/%s", server.URL, productUID, packageUID, object)
	r, err := http.Get(finalURL)
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, r.StatusCode)
	assert.Equal(t, []byte("object_content"), bodyContent)

	lam.AssertExpectations(t)
}

func TestGetObjectRouteWithUhupkg(t *testing.T) {
	memFs := afero.NewOsFs()

	la := &libarchive.LibArchive{}

	productUID := "548873c4d4e8e751fdd46c38a3d5a8656cf87bf27a404f346ad58086f627a4ea"
	packageUID := utils.DataSha256sum([]byte(ValidJSONMetadata))
	object := "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa"

	// setup filesystem
	tarballPath, err := generateUhupkg(memFs, true)
	assert.NoError(t, err)

	testPath, err := afero.TempDir(memFs, "", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	pkgpath := path.Join(testPath, "name.uhupkg")

	os.Link(tarballPath, pkgpath)

	// setup server
	sb, err := NewServerBackend(la, testPath)
	assert.NoError(t, err)

	router := NewBackendRouter(sb)
	server := httptest.NewServer(router.HTTPRouter)

	err = sb.ProcessDirectory()
	assert.NoError(t, err)

	// do the request
	finalURL := fmt.Sprintf("%s/products/%s/packages/%s/objects/%s", server.URL, productUID, packageUID, object)
	r, err := http.Get(finalURL)
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, r.StatusCode)
	assert.Equal(t, "content1", string(bodyContent))
}

func TestGetObjectRouteWithUhupkgNoPackageSelectedError(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	memFs := afero.NewOsFs()

	productUID := "548873c4d4e8e751fdd46c38a3d5a8656cf87bf27a404f346ad58086f627a4ea"
	packageUID := utils.DataSha256sum([]byte(ValidJSONMetadata))
	object := "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa"

	// setup filesystem
	testPath, err := afero.TempDir(memFs, "", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	// setup server
	sb, err := NewServerBackend(lam, testPath)
	assert.NoError(t, err)

	router := NewBackendRouter(sb)
	server := httptest.NewServer(router.HTTPRouter)

	// do the request
	finalURL := fmt.Sprintf("%s/products/%s/packages/%s/objects/%s", server.URL, productUID, packageUID, object)
	r, err := http.Get(finalURL)
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
	assert.Equal(t, "500 internal server error\n", string(bodyContent))

	lam.AssertExpectations(t)
}

func TestGetObjectRouteWithUhupkgExtractError(t *testing.T) {
	memFs := afero.NewOsFs()

	la := &libarchive.LibArchive{}

	productUID := "548873c4d4e8e751fdd46c38a3d5a8656cf87bf27a404f346ad58086f627a4ea"
	packageUID := utils.DataSha256sum([]byte(ValidJSONMetadata))
	object := "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa"

	// setup filesystem
	tarballPath, err := generateUhupkg(memFs, false) // with error
	assert.NoError(t, err)

	testPath, err := afero.TempDir(memFs, "", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	pkgpath := path.Join(testPath, "name.uhupkg")

	os.Link(tarballPath, pkgpath)

	// setup server
	sb, err := NewServerBackend(la, testPath)
	assert.NoError(t, err)

	router := NewBackendRouter(sb)
	server := httptest.NewServer(router.HTTPRouter)

	err = sb.ProcessDirectory()
	assert.NoError(t, err)

	// do the request
	finalURL := fmt.Sprintf("%s/products/%s/packages/%s/objects/%s", server.URL, productUID, packageUID, object)
	r, err := http.Get(finalURL)
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
	assert.Equal(t, "500 internal server error\n", string(bodyContent))
}

func TestReportRoute(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	// setup filesystem
	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, metadata.UpdateMetadataFilename)

	err = ioutil.WriteFile(updateMetadataFilePath, []byte(ValidJSONMetadata), 0666)
	assert.NoError(t, err)

	// setup server
	sb, err := NewServerBackend(lam, testPath)
	assert.NoError(t, err)

	router := NewBackendRouter(sb)
	server := httptest.NewServer(router.HTTPRouter)

	err = sb.ProcessDirectory()
	assert.NoError(t, err)

	// do the request
	reportData := []byte("{\"status\": \"downloading\", \"package-uid\": \"puid\"}")
	r, err := http.Post(server.URL+"/report", "application/json", bytes.NewBuffer(reportData))
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, r.StatusCode)
	assert.Equal(t, []byte(""), bodyContent)

	lam.AssertExpectations(t)
}

func TestReportRouteWithInvalidReportData(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}

	// setup filesystem
	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, metadata.UpdateMetadataFilename)

	err = ioutil.WriteFile(updateMetadataFilePath, []byte(ValidJSONMetadata), 0666)
	assert.NoError(t, err)

	// setup server
	sb, err := NewServerBackend(lam, testPath)
	assert.NoError(t, err)

	router := NewBackendRouter(sb)
	server := httptest.NewServer(router.HTTPRouter)

	err = sb.ProcessDirectory()
	assert.NoError(t, err)

	// do the request
	r, err := http.Post(server.URL+"/report", "application/json", bytes.NewBuffer([]byte("{\"content\": ")))
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
	assert.Equal(t, []byte("500 internal server error\n"), bodyContent)

	lam.AssertExpectations(t)
}

func generateUhupkg(fsBackend afero.Fs, valid bool) (string, error) {
	zipPath := "/tmp/output.zip"
	file, err := fsBackend.Create(zipPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	zw := zip.NewWriter(file)

	var files = []struct {
		Name, Body string
	}{
		{"d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa", "content1"},
		{"metadata", ValidJSONMetadata},
	}

	for _, file := range files {
		if !valid && file.Name != "metadata" {
			// if it's an invalid file, write only the metadata
			continue
		}

		f, err := zw.Create(file.Name)

		if err != nil {
			return "", err
		}

		_, err = f.Write([]byte(file.Body))
		if err != nil {
			return "", err
		}
	}

	err = zw.Close()
	if err != nil {
		return "", err
	}

	return zipPath, nil
}
