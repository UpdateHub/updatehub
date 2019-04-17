package server

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/UpdateHub/updatehub/libarchive"
)

type UpdatePackage struct {
	updateMetadata []byte
	signature      []byte
	file           *os.File
}

func NewUpdatePackage(f *os.File) (*UpdatePackage, error) {
	updateMetadata, signature, err := parseUpdatePackage(f)
	if err != nil {
		return nil, err
	}

	return &UpdatePackage{
		updateMetadata: updateMetadata,
		signature:      signature,
		file:           f,
	}, nil
}

func fetchUpdatePackage(target *url.URL) (*UpdatePackage, error) {
	res, err := http.Get(target.String())
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("failed")
	}

	out, err := ioutil.TempFile("", "uhupkg")
	if err != nil {
		return nil, err
	}

	defer out.Close()

	_, err = io.Copy(out, res.Body)
	if err != nil {
		return nil, err
	}

	return NewUpdatePackage(out)
}

func parseUpdatePackage(f *os.File) ([]byte, []byte, error) {
	la := libarchive.LibArchive{}

	reader, err := libarchive.NewReader(la, f.Name(), 10240)
	if err != nil {
		return nil, nil, err
	}

	defer reader.Free()

	metadataBuff := bytes.NewBuffer(nil)
	err = reader.ExtractFile("metadata", metadataBuff)
	if err != nil {
		return nil, nil, err
	}

	reader, err = libarchive.NewReader(la, f.Name(), 10240)
	if err != nil {
		return metadataBuff.Bytes(), nil, err
	}

	signatureBuff := bytes.NewBuffer(nil)
	err = reader.ExtractFile("signature", signatureBuff)
	if err != nil {
		return metadataBuff.Bytes(), nil, err
	}

	return metadataBuff.Bytes(), signatureBuff.Bytes(), nil
}
