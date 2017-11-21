package utils

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLimitReader(t *testing.T) {
	n := int64(3)
	rd := bytes.NewReader([]byte("bytes"))
	lr := LimitReader(rd, n)

	assert.Equal(t, rd, lr.(*LimitedReader).R)
	assert.Equal(t, n, lr.(*LimitedReader).N)
}

func TestLimitReaderRead(t *testing.T) {
	n := int64(3)
	rd := bytes.NewReader([]byte("bytes"))
	lr := LimitReader(rd, n)

	limitedBytes, err := ioutil.ReadAll(lr)
	assert.NoError(t, err)

	bytes := make([]byte, n)

	rd.Seek(0, io.SeekStart)

	_, err = rd.Read(bytes)

	assert.NoError(t, err)
	assert.Equal(t, int64(0), lr.(*LimitedReader).N)
	assert.Equal(t, bytes, limitedBytes)
}

func TestLimitReaderSeek(t *testing.T) {
	n := int64(3)
	skip := int64(1)
	rd := bytes.NewReader([]byte("bytes"))
	lr := LimitReader(rd, n)

	_, err := ioutil.ReadAll(lr)
	assert.NoError(t, err)

	lr.Seek(skip, io.SeekStart)

	assert.Equal(t, n, lr.(*LimitedReader).N)

	limitedBytes, err := ioutil.ReadAll(lr)
	assert.NoError(t, err)

	bytes := make([]byte, n)

	rd.Seek(skip, io.SeekStart)

	_, err = rd.Read(bytes)

	assert.NoError(t, err)
	assert.Equal(t, int64(0), lr.(*LimitedReader).N)
	assert.Equal(t, bytes, limitedBytes)
}
