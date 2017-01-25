package utils

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// PLEASE FIXME: avoid time.Sleep

type SlowReader struct {
	data  []byte
	index int64
}

func (r *SlowReader) Read(b []byte) (n int, err error) {
	if r.index >= int64(len(r.data)) {
		err = io.EOF
		return
	}

	n = copy(b, r.data[r.index:r.index+1])

	r.index++

	time.Sleep(time.Second)

	return
}

func NewSlowReader(data string) *SlowReader {
	return &SlowReader{
		data: []byte(data),
	}
}

func TestCopyTimeoutHasReached(t *testing.T) {
	rd := NewSlowReader("123")

	var buff bytes.Buffer

	wr := bufio.NewWriter(&buff)

	cancel := make(chan bool)

	cancelled, err := Copy(wr, rd, time.Millisecond, cancel)

	assert.False(t, cancelled)
	if assert.Error(t, err) {
		assert.Equal(t, errors.New("timeout"), err)
	}

	assert.Empty(t, buff.Bytes())
}

func TestCancelCopy(t *testing.T) {
	rd := NewSlowReader("123")

	var buff bytes.Buffer

	wr := bufio.NewWriter(&buff)

	var cancelled bool
	var err error

	cancel := make(chan bool)
	wait := make(chan bool)

	go func() {
		cancelled, err = Copy(wr, rd, time.Minute, cancel)
		wait <- false
	}()

	go func() {
		time.Sleep(time.Second * 2)
		cancel <- true
	}()

	<-wait

	assert.True(t, cancelled)
	assert.NoError(t, err)
	assert.NotEmpty(t, buff.Bytes())
}
