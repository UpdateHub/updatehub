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

type TimedReader struct {
	data        []byte
	index       int64
	idleTimeout time.Duration
	onRead      func()
}

func (r *TimedReader) Read(b []byte) (n int, err error) {
	if r.index >= int64(len(r.data)) {
		err = io.EOF
		return
	}

	n = copy(b, r.data[r.index:r.index+1])

	r.index++

	time.Sleep(r.idleTimeout)

	r.onRead()

	return
}

func NewTimedReader(data string) *TimedReader {
	return &TimedReader{
		data:        []byte(data),
		idleTimeout: time.Millisecond,
		onRead:      func() {},
	}
}

func TestCopy(t *testing.T) {
	data := "123"

	buff := bytes.NewBuffer(nil)

	rd := NewTimedReader(data)
	wr := bufio.NewWriter(buff)

	cancelled, err := Copy(wr, rd, time.Minute, nil, ChunkSize, -1)

	err = wr.Flush()
	assert.NoError(t, err)

	assert.NoError(t, err)
	assert.False(t, cancelled)
	assert.Equal(t, data, buff.String())
}

func TestCopyTimeoutHasReached(t *testing.T) {
	rd := NewTimedReader("123")

	rd.idleTimeout = time.Minute

	buff := bytes.NewBuffer(nil)
	wr := bufio.NewWriter(buff)

	cancel := make(chan bool)

	cancelled, err := Copy(wr, rd, time.Millisecond, cancel, ChunkSize, -1)
	assert.False(t, cancelled)
	if !assert.Error(t, err) {
		assert.Equal(t, errors.New("timeout"), err)
	}

	err = wr.Flush()
	assert.NoError(t, err)

	assert.Empty(t, buff.Bytes())
}

func TestCancelCopy(t *testing.T) {
	rd := NewTimedReader("123")

	buff := bytes.NewBuffer(nil)
	wr := bufio.NewWriter(buff)

	var cancelled bool
	var err error

	cancel := make(chan bool)
	wait := make(chan bool)

	go func() {
		cancelled, err = Copy(wr, rd, time.Minute, cancel, ChunkSize, -1)
		wait <- false
	}()

	var ticks int
	rd.onRead = func() {
		if ticks == 2 {
			cancel <- true
		}

		ticks++
	}

	<-wait

	assert.True(t, cancelled)
	assert.NoError(t, err)

	err = wr.Flush()
	assert.NoError(t, err)

	assert.NotEmpty(t, buff.Bytes())
}
