/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package utils

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"syscall"
	"testing"
	"time"

	"github.com/UpdateHub/updatehub/libarchive"
	"github.com/UpdateHub/updatehub/testsmocks/filemock"
	"github.com/UpdateHub/updatehub/testsmocks/filesystemmock"
	"github.com/UpdateHub/updatehub/testsmocks/libarchivemock"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

	eio := ExtendedIO{}
	cancelled, err := eio.Copy(wr, rd, time.Minute, nil, ChunkSize, 0, -1, false)

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

	eio := ExtendedIO{}
	cancelled, err := eio.Copy(wr, rd, time.Millisecond, cancel, ChunkSize, 0, -1, false)
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

	var ticks int
	rd.onRead = func() {
		if ticks == 2 {
			cancel <- true
		}

		ticks++
	}

	go func() {
		eio := ExtendedIO{}
		cancelled, err = eio.Copy(wr, rd, time.Minute, cancel, ChunkSize, 0, -1, false)
		wait <- false
	}()

	<-wait

	assert.True(t, cancelled)
	assert.NoError(t, err)

	err = wr.Flush()
	assert.NoError(t, err)

	assert.NotEmpty(t, buff.Bytes())
}

func compressData(decompressedData []byte, compressor string) ([]byte, error) {
	tempDecompressed, err := ioutil.TempFile("", "copy-test")
	if err != nil {
		return []byte(nil), err
	}
	defer os.Remove(tempDecompressed.Name())

	_, err = tempDecompressed.Write(decompressedData)
	if err != nil {
		return []byte(nil), err
	}

	err = tempDecompressed.Close()
	if err != nil {
		return []byte(nil), err
	}

	tempCompressed, err := ioutil.TempFile("", "copy-test")
	if err != nil {
		return []byte(nil), err
	}
	defer os.Remove(tempCompressed.Name())

	err = tempCompressed.Close()
	if err != nil {
		return []byte(nil), err
	}

	cl := CmdLine{}

	_, err = cl.Execute(fmt.Sprintf("sh -c \"%s -c %s > %s\"", compressor, tempDecompressed.Name(), tempCompressed.Name()))
	if err != nil {
		return []byte(nil), err
	}

	return ioutil.ReadFile(tempCompressed.Name())
}

func TestCopyFileIntegration(t *testing.T) {
	testCases := []struct {
		Name                      string
		SourceFileContent         []byte
		ExistingTargetFileContent []byte
		ExpectedTargetFileContent []byte
		ChunkSize                 int
		Skip                      int
		Seek                      int
		Count                     int
		Truncate                  bool
		Compressed                bool
	}{
		{
			"Success",
			[]byte("content"),
			[]byte("targetcontent"),
			[]byte("content"),
			128,
			0,
			0,
			-1,
			true,
			false,
		},
		{
			"ZeroBytesSourceFile",
			[]byte(""),
			[]byte("targetcontent"),
			[]byte(""),
			128,
			0,
			0,
			-1,
			true,
			false,
		},
		{
			"WithSkipAndSeek",
			[]byte("56789_source_56789"),
			[]byte("01234!_dest_01234!"),
			[]byte("01234!_dest_source_56789"),
			2,
			3,
			6,
			-1,
			false,
			false,
		},
		{
			"SuccessCompressed",
			[]byte("content"),
			[]byte("targetcontent"),
			[]byte("content"),
			128,
			0,
			0,
			-1,
			true,
			true,
		},
		{
			"CompressedWithZeroBytesSourceFile",
			[]byte(""),
			[]byte("targetcontent"),
			[]byte(""),
			128,
			0,
			0,
			-1,
			true,
			true,
		},
		{
			"CompressedWithSkipAndSeek",
			[]byte("56789_source_56789"),
			[]byte("01234!_dest_01234!"),
			[]byte("01234!_dest_source_56789"),
			2,
			3,
			6,
			-1,
			false,
			true,
		},
		{
			"CompressedWithCount",
			[]byte("source_content"),
			[]byte(""),
			[]byte("source"),
			2,
			0,
			0,
			3,
			false,
			true,
		},
		{
			"CompressedWithTruncate",
			[]byte("source"),
			[]byte("target_content_bigger_than_source"),
			[]byte("source"),
			128,
			0,
			0,
			-1,
			true,
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			memFs := afero.NewOsFs()

			testPath, err := afero.TempDir(memFs, "", "CopyFile-test")
			assert.NoError(t, err)
			defer memFs.RemoveAll(testPath)

			sourcePath := path.Join(testPath, "source.txt")
			source, err := memFs.Create(sourcePath)
			assert.NoError(t, err)

			if tc.Compressed {
				content, err := compressData(tc.SourceFileContent, "gzip")
				assert.NoError(t, err)

				_, err = source.Write(content)
				assert.NoError(t, err)
			} else {
				_, err = source.Write(tc.SourceFileContent)
				assert.NoError(t, err)
			}

			err = source.Close()
			assert.NoError(t, err)

			targetPath := path.Join(testPath, "target.txt")
			err = ioutil.WriteFile(targetPath, tc.ExistingTargetFileContent, 0666)
			assert.NoError(t, err)

			eio := ExtendedIO{}
			err = eio.CopyFile(memFs, libarchive.LibArchive{}, sourcePath, targetPath, tc.ChunkSize,
				tc.Skip, tc.Seek, tc.Count, tc.Truncate, tc.Compressed)
			assert.NoError(t, err)

			pathExists, err := afero.Exists(memFs, targetPath)
			assert.True(t, pathExists)
			assert.NoError(t, err)

			data, err := afero.ReadFile(memFs, targetPath)
			assert.NoError(t, err)
			assert.Equal(t, tc.ExpectedTargetFileContent, data)
		})
	}
}

func TestCopyFileWithSuccess(t *testing.T) {
	const (
		chunkSize = 128 * 1024
	)

	sourceMock := &filemock.FileMock{}
	sourceContent := []uint8("test")
	sourceMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		// return the whole "sourceContent" since chunkSize is bigger
		// than the content
		copy(arg, sourceContent)
	}).Return(len(sourceContent), nil).Once()
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Return(0, io.EOF).Once()
	sourceMock.On("Close").Return(nil)

	targetMock := &filemock.FileMock{}
	targetContent := []uint8("")
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		targetContent = arg
	}).Return(len(targetContent), nil).Once()
	targetMock.On("Close").Return(nil)

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, &libarchivemock.LibArchiveMock{}, "source.txt", "target.txt", chunkSize,
		0, 0, -1, true, false)
	assert.NoError(t, err)

	assert.Equal(t, sourceContent, targetContent)

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCopyFileWithSuccessWithMultipleChunks(t *testing.T) {
	const (
		chunkSize = 2
	)

	sourceMock := &filemock.FileMock{}
	sourceContent := []uint8("test")
	sourceMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		// return the first "chunkSize" bytes of "sourceContent"
		copy(arg, sourceContent[:1*chunkSize])
	}).Return(chunkSize, nil).Once()
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		// then return the following "chunkSize" bytes of "sourceContent"
		copy(arg, sourceContent[1*chunkSize:2*chunkSize])
	}).Return(chunkSize, nil).Once()
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Return(0, io.EOF).Once()
	sourceMock.On("Close").Return(nil)

	targetMock := &filemock.FileMock{}
	targetContent := []uint8("")
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		targetContent = append(targetContent, arg...)
	}).Return(chunkSize, nil).Once()
	targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		targetContent = append(targetContent, arg...)
	}).Return(chunkSize, nil).Once()
	targetMock.On("Close").Return(nil)

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, &libarchivemock.LibArchiveMock{}, "source.txt", "target.txt", chunkSize,
		0, 0, -1, true, false)
	assert.NoError(t, err)

	assert.Equal(t, sourceContent, targetContent)

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCopyFileWithSuccessUsingSkipAndSeek(t *testing.T) {
	const (
		skip      = 3
		seek      = 1
		chunkSize = 4
	)

	sourceMock := &filemock.FileMock{}
	readContent := []uint8("test")
	sourceMock.On("Seek", int64(skip*chunkSize), io.SeekStart).Return(int64(skip*chunkSize), nil)
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		// return the whole "sourceContent" since chunkSize is bigger
		// than the content
		copy(arg, readContent)
	}).Return(len(readContent), nil).Once()
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Return(0, io.EOF).Once()
	sourceMock.On("Close").Return(nil)

	targetMock := &filemock.FileMock{}
	writeContent := []uint8("")
	targetMock.On("Seek", int64(seek*chunkSize), io.SeekStart).Return(int64(skip*chunkSize), nil)
	targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		writeContent = arg
	}).Return(len(writeContent), nil).Once()
	targetMock.On("Close").Return(nil)

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, &libarchivemock.LibArchiveMock{}, "source.txt", "target.txt", chunkSize,
		skip, seek, -1, true, false)
	assert.NoError(t, err)

	assert.Equal(t, readContent, writeContent)

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCopyFileUsingCount(t *testing.T) {
	const (
		chunkSize = 1
		count     = 3
	)

	sourceMock := &filemock.FileMock{}
	sourceContent := []uint8("test")
	sourceMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)

	for i := 0; i < count; i++ {
		i := i
		sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
			arg := args.Get(0).([]uint8)
			copy(arg, sourceContent[(i)*chunkSize:(i+1)*chunkSize])
		}).Return(chunkSize, nil).Once()
	}

	sourceMock.On("Close").Return(nil)

	targetMock := &filemock.FileMock{}
	targetContent := []uint8("")
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)

	for i := 0; i < count; i++ {
		targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
			arg := args.Get(0).([]uint8)
			targetContent = append(targetContent, arg...)
		}).Return(chunkSize, nil).Once()
	}

	targetMock.On("Close").Return(nil)

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, &libarchivemock.LibArchiveMock{}, "source.txt", "target.txt", chunkSize,
		0, 0, count, true, false)
	assert.NoError(t, err)

	assert.Equal(t, sourceContent[:count*chunkSize], targetContent)

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCopyFileUsingNegativeCount(t *testing.T) {
	// negative count means the whole file
	const (
		chunkSize = 1
		count     = -1
	)

	sourceMock := &filemock.FileMock{}
	sourceContent := []uint8("test")
	sourceMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)

	for i := 0; i < len(sourceContent); i++ {
		i := i
		sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
			arg := args.Get(0).([]uint8)
			copy(arg, sourceContent[(i)*chunkSize:(i+1)*chunkSize])
		}).Return(chunkSize, nil).Once()
	}
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Return(0, io.EOF).Once()

	sourceMock.On("Close").Return(nil)

	targetMock := &filemock.FileMock{}
	targetContent := []uint8("")
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)

	for i := 0; i < len(sourceContent); i++ {
		targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
			arg := args.Get(0).([]uint8)
			targetContent = append(targetContent, arg...)
		}).Return(chunkSize, nil).Once()
	}

	targetMock.On("Close").Return(nil)

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, &libarchivemock.LibArchiveMock{}, "source.txt", "target.txt", chunkSize,
		0, 0, count, true, false)
	assert.NoError(t, err)

	assert.Equal(t, sourceContent, targetContent)

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCopyFileNotUsingTruncate(t *testing.T) {
	const (
		truncate = false
	)

	sourceMock := &filemock.FileMock{}
	sourceContent := []uint8("test")
	sourceMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		copy(arg, sourceContent)
	}).Return(len(sourceContent), nil).Once()
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Return(0, io.EOF).Once()
	sourceMock.On("Close").Return(nil)

	targetMock := &filemock.FileMock{}
	targetContent := []uint8("")
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		targetContent = append(targetContent, arg...)
	}).Return(len(targetContent), nil).Once()
	targetMock.On("Close").Return(nil)

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE, os.FileMode(0666)).Return(targetMock, nil)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, &libarchivemock.LibArchiveMock{}, "source.txt", "target.txt", ChunkSize,
		0, 0, -1, truncate, false)
	assert.NoError(t, err)

	assert.Equal(t, sourceContent, targetContent)

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCopyFileWithOpenError(t *testing.T) {
	targetMock := &filemock.FileMock{}
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Close").Return(nil)

	pathError := &os.PathError{
		Op:   "open",
		Path: "source.txt",
		Err:  syscall.ENOSPC,
	}

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)
	fom.On("Open", "source.txt").Return((*filemock.FileMock)(nil), pathError)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, &libarchivemock.LibArchiveMock{}, "source.txt", "target.txt", ChunkSize,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "open source.txt: no space left on device")

	fom.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCopyFileWithOpenFileError(t *testing.T) {
	pathError := &os.PathError{
		Op:   "open",
		Path: "target.txt",
		Err:  syscall.ENOSPC,
	}

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return((*filemock.FileMock)(nil), pathError)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, &libarchivemock.LibArchiveMock{}, "source.txt", "target.txt", ChunkSize,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "open target.txt: no space left on device")

	fom.AssertExpectations(t)
}

func TestCopyFileWithReadError(t *testing.T) {
	sourceMock := &filemock.FileMock{}
	sourceMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Return(0, io.ErrClosedPipe).Once()
	sourceMock.On("Close").Return(nil)

	targetMock := &filemock.FileMock{}
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Close").Return(nil)

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, &libarchivemock.LibArchiveMock{}, "source.txt", "target.txt", ChunkSize,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "io: read/write on closed pipe")

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCopyFileWithWriteError(t *testing.T) {
	sourceMock := &filemock.FileMock{}
	sourceContent := []uint8("test")
	sourceMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		copy(arg, sourceContent)
	}).Return(len(sourceContent), nil).Once()
	sourceMock.On("Close").Return(nil)

	targetMock := &filemock.FileMock{}
	targetContent := []uint8("")
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		targetContent = arg
	}).Return(0, io.ErrClosedPipe).Once()
	targetMock.On("Close").Return(nil)

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, &libarchivemock.LibArchiveMock{}, "source.txt", "target.txt", ChunkSize,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "io: read/write on closed pipe")

	assert.Equal(t, sourceContent, targetContent)

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCopyFileWithZeroedChunkSize(t *testing.T) {
	const (
		chunkSize = 0
	)

	sourceMock := &filemock.FileMock{}
	sourceMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	sourceMock.On("Close").Return(nil)

	targetMock := &filemock.FileMock{}
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Close").Return(nil)

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, &libarchivemock.LibArchiveMock{}, "source.txt", "target.txt", chunkSize,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "Copy error: chunkSize can't be less than 1")

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCopyFileWithNegativeChunkSize(t *testing.T) {
	const (
		chunkSize = -1
	)

	sourceMock := &filemock.FileMock{}
	sourceMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	sourceMock.On("Close").Return(nil)

	targetMock := &filemock.FileMock{}
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Close").Return(nil)

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, &libarchivemock.LibArchiveMock{}, "source.txt", "target.txt", chunkSize,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "Copy error: chunkSize can't be less than 1")

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCustomCopyFileWithSkipError(t *testing.T) {
	const (
		chunkSize = 128 * 1024
	)

	sourceMock := &filemock.FileMock{}
	sourceMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), fmt.Errorf("Seek: invalid whence"))
	sourceMock.On("Close").Return(nil)

	targetMock := &filemock.FileMock{}
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Close").Return(nil)

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, &libarchivemock.LibArchiveMock{}, "source.txt", "target.txt", chunkSize,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "Seek: invalid whence")

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCopyFileWithSeekError(t *testing.T) {
	const (
		chunkSize = 128 * 1024
	)

	targetMock := &filemock.FileMock{}
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), fmt.Errorf("Seek: invalid whence"))
	targetMock.On("Close").Return(nil)

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, &libarchivemock.LibArchiveMock{}, "source.txt", "target.txt", chunkSize,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "Seek: invalid whence")

	fom.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCopyFileWithSuccessUsingLibarchive(t *testing.T) {
	const (
		chunkSize  = 128 * 1024
		sourcePath = "source.gz"
	)

	libarchiveMock := &libarchivemock.LibArchiveMock{}

	sourceContent := []uint8("test")

	a := libarchive.LibArchive{}.NewRead()
	libarchiveMock.On("NewRead").Return(a)
	libarchiveMock.On("ReadSupportFilterAll", a)
	libarchiveMock.On("ReadSupportFormatRaw", a)
	libarchiveMock.On("ReadSupportFormatEmpty", a)
	libarchiveMock.On("ReadOpenFileName", a, sourcePath, chunkSize).Return(nil)
	libarchiveMock.On("ReadNextHeader", a, mock.AnythingOfType("*libarchive.ArchiveEntry")).Return(nil)
	libarchiveMock.On("ReadData", a, mock.AnythingOfType("[]uint8"), chunkSize).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		// return the whole "sourceContent" since chunkSize is bigger
		// than the content
		copy(arg, sourceContent)
	}).Return(len(sourceContent), nil).Once()
	libarchiveMock.On("ReadData", a, mock.AnythingOfType("[]uint8"), chunkSize).Return(0, nil).Once()
	libarchiveMock.On("ReadFree", a)

	targetMock := &filemock.FileMock{}
	targetContent := []uint8("")
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		targetContent = arg
	}).Return(len(targetContent), nil).Once()
	targetMock.On("Close").Return(nil)

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, libarchiveMock, sourcePath, "target.txt", chunkSize,
		0, 0, -1, true, true)
	assert.NoError(t, err)

	assert.Equal(t, sourceContent, targetContent)

	fom.AssertExpectations(t)
	libarchiveMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCopyFileWithLibarchiveReadOpenFileNameError(t *testing.T) {
	const (
		chunkSize  = 128 * 1024
		sourcePath = "source.gz"
	)

	libarchiveMock := &libarchivemock.LibArchiveMock{}

	a := libarchive.LibArchive{}.NewRead()
	libarchiveMock.On("NewRead").Return(a)
	libarchiveMock.On("ReadSupportFilterAll", a)
	libarchiveMock.On("ReadSupportFormatRaw", a)
	libarchiveMock.On("ReadSupportFormatEmpty", a)
	libarchiveMock.On("ReadOpenFileName", a, sourcePath, chunkSize).Return(fmt.Errorf("Failed to open '%s'", sourcePath))
	libarchiveMock.On("ReadFree", a)

	targetMock := &filemock.FileMock{}
	targetContent := []uint8("")
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Close").Return(nil)

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, libarchiveMock, sourcePath, "target.txt", chunkSize,
		0, 0, -1, true, true)
	assert.EqualError(t, err, fmt.Sprintf("Failed to open '%s'", sourcePath))

	assert.Equal(t, []byte(""), targetContent)

	fom.AssertExpectations(t)
	libarchiveMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCopyFileWithLibarchiveReadNextHeaderError(t *testing.T) {
	const (
		chunkSize  = 128 * 1024
		sourcePath = "source.gz"
	)

	libarchiveMock := &libarchivemock.LibArchiveMock{}

	a := libarchive.LibArchive{}.NewRead()
	libarchiveMock.On("NewRead").Return(a)
	libarchiveMock.On("ReadSupportFilterAll", a)
	libarchiveMock.On("ReadSupportFormatRaw", a)
	libarchiveMock.On("ReadSupportFormatEmpty", a)
	libarchiveMock.On("ReadOpenFileName", a, sourcePath, chunkSize).Return(nil)
	libarchiveMock.On("ReadNextHeader", a, mock.AnythingOfType("*libarchive.ArchiveEntry")).Return(fmt.Errorf("Mock emulated error"))
	libarchiveMock.On("ReadFree", a)

	targetMock := &filemock.FileMock{}
	targetContent := []uint8("")
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Close").Return(nil)

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, libarchiveMock, sourcePath, "target.txt", chunkSize,
		0, 0, -1, true, true)
	assert.EqualError(t, err, "Mock emulated error")

	assert.Equal(t, []byte(""), targetContent)

	fom.AssertExpectations(t)
	libarchiveMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCopyFileWithLibarchiveReadDataError(t *testing.T) {
	const (
		chunkSize  = 128 * 1024
		sourcePath = "source.gz"
	)

	libarchiveMock := &libarchivemock.LibArchiveMock{}

	a := libarchive.LibArchive{}.NewRead()
	libarchiveMock.On("NewRead").Return(a)
	libarchiveMock.On("ReadSupportFilterAll", a)
	libarchiveMock.On("ReadSupportFormatRaw", a)
	libarchiveMock.On("ReadSupportFormatEmpty", a)
	libarchiveMock.On("ReadOpenFileName", a, sourcePath, chunkSize).Return(nil)
	libarchiveMock.On("ReadNextHeader", a, mock.AnythingOfType("*libarchive.ArchiveEntry")).Return(nil)
	libarchiveMock.On("ReadData", a, mock.AnythingOfType("[]uint8"), chunkSize).Return(-30, fmt.Errorf("Mock emulated error")).Once()
	libarchiveMock.On("ReadFree", a)

	targetMock := &filemock.FileMock{}
	targetContent := []uint8("")
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Close").Return(nil)

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, libarchiveMock, sourcePath, "target.txt", chunkSize,
		0, 0, -1, true, true)
	assert.EqualError(t, err, "Mock emulated error")

	assert.Equal(t, []byte(""), targetContent)

	fom.AssertExpectations(t)
	libarchiveMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCopyFileWithLibarchiveWriteError(t *testing.T) {
	const (
		chunkSize  = 128 * 1024
		sourcePath = "source.gz"
	)

	libarchiveMock := &libarchivemock.LibArchiveMock{}

	sourceContent := []uint8("test")
	a := libarchive.LibArchive{}.NewRead()
	libarchiveMock.On("NewRead").Return(a)
	libarchiveMock.On("ReadSupportFilterAll", a)
	libarchiveMock.On("ReadSupportFormatRaw", a)
	libarchiveMock.On("ReadSupportFormatEmpty", a)
	libarchiveMock.On("ReadOpenFileName", a, sourcePath, chunkSize).Return(nil)
	libarchiveMock.On("ReadNextHeader", a, mock.AnythingOfType("*libarchive.ArchiveEntry")).Return(nil)
	libarchiveMock.On("ReadData", a, mock.AnythingOfType("[]uint8"), chunkSize).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		// return the whole "sourceContent" since chunkSize is bigger
		// than the content
		copy(arg, sourceContent)
	}).Return(len(sourceContent), nil).Once()
	libarchiveMock.On("ReadFree", a)

	targetMock := &filemock.FileMock{}
	targetContent := []uint8("")
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		targetContent = arg
	}).Return(0, io.ErrClosedPipe).Once()
	targetMock.On("Close").Return(nil)

	fom := &filesystemmock.FileSystemBackendMock{}
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	eio := ExtendedIO{}
	err := eio.CopyFile(fom, libarchiveMock, sourcePath, "target.txt", chunkSize,
		0, 0, -1, true, true)
	assert.EqualError(t, err, "io: read/write on closed pipe")

	assert.Equal(t, sourceContent, targetContent)

	fom.AssertExpectations(t)
	libarchiveMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

/*
FIXME: cases missing

uncompressed and compressed
- keep file attributes test

errors
- when applying attributes (permission)
*/

func TestCopyToProcessStdinIntegration(t *testing.T) {
	testCases := []struct {
		Name                      string
		SourceFileContent         []byte
		CmdLine                   string
		ExistingTargetFileContent []byte
		ExpectedError             error
		ExpectedTargetFileContent []byte
		Compressed                bool
	}{
		{
			"SuccessNonCompressed",
			[]byte("some_filler_data"),
			"tee %s",
			[]byte("old_content"),
			nil,
			[]byte("some_filler_data"),
			false,
		},
		{
			"SuccessCompressed",
			[]byte("some_filler_data"),
			"tee %s",
			[]byte("old_content"),
			nil,
			[]byte("some_filler_data"),
			true,
		},
		{
			"WithCmdLineError",
			[]byte("some_filler_data"),
			"non-existant-command %s",
			[]byte("old_content"),
			fmt.Errorf(`exec: "non-existant-command": executable file not found in $PATH`),
			[]byte("old_content"),
			false,
		},
		{
			"WithEmptyCompressedSourceFile",
			[]byte(""),
			"tee %s",
			[]byte("old_content"),
			nil,
			[]byte(""),
			true,
		},
		{
			"WithInvalidCmdLine",
			[]byte(""),
			`tee "%s`,
			[]byte("old_content"),
			fmt.Errorf("invalid command line string"),
			[]byte("old_content"),
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			osFs := afero.NewOsFs()

			testPath, err := afero.TempDir(osFs, "", "CopyToProcessstdin-test")
			assert.NoError(t, err)
			defer osFs.RemoveAll(testPath)

			sourcePath := path.Join(testPath, "source.txt")
			source, err := osFs.Create(sourcePath)
			assert.NoError(t, err)

			if tc.Compressed {
				content, err := compressData(tc.SourceFileContent, "gzip")
				assert.NoError(t, err)

				_, err = source.Write(content)
				assert.NoError(t, err)
			} else {
				_, err = source.Write(tc.SourceFileContent)
				assert.NoError(t, err)
			}

			err = source.Close()
			assert.NoError(t, err)

			targetPath := path.Join(testPath, "target.txt")
			err = ioutil.WriteFile(targetPath, tc.ExistingTargetFileContent, 0666)
			assert.NoError(t, err)

			processCmdline := fmt.Sprintf(tc.CmdLine, targetPath)

			eio := ExtendedIO{}
			err = eio.CopyToProcessStdin(osFs, &libarchive.LibArchive{}, sourcePath, processCmdline, tc.Compressed)
			if tc.ExpectedError == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.ExpectedError.Error())
			}

			pathExists, err := afero.Exists(osFs, targetPath)
			assert.True(t, pathExists)
			assert.NoError(t, err)

			data, err := afero.ReadFile(osFs, targetPath)
			assert.NoError(t, err)
			assert.Equal(t, tc.ExpectedTargetFileContent, data)
		})
	}
}

func TestCopyToProcessStdinWithProcessExitError(t *testing.T) {
	testPath, err := ioutil.TempDir("", "CopyToProcessStdin-test")
	assert.Nil(t, err)
	defer os.RemoveAll(testPath)

	binaryContent := `#!/bin/sh
read stuff
echo "stdout string $stuff"
exit 1
`
	fakeCmdPath := path.Join(testPath, "binary")
	fakeCmdFile, err := os.Create(fakeCmdPath)
	assert.Nil(t, err)
	err = os.Chmod(fakeCmdPath, 0777)
	assert.Nil(t, err)
	_, err = fakeCmdFile.WriteString(binaryContent)
	assert.Nil(t, err)
	err = fakeCmdFile.Close()
	assert.Nil(t, err)

	cmdString := fakeCmdPath + " arg1"

	osFs := afero.NewOsFs()

	sourcePath := path.Join(testPath, "target.txt")
	err = ioutil.WriteFile(sourcePath, []byte("existing_content"), 0666)
	assert.NoError(t, err)

	eio := ExtendedIO{}
	err = eio.CopyToProcessStdin(osFs, &libarchive.LibArchive{}, sourcePath, cmdString, false)
	assert.EqualError(t, err, "exit status 1")
}
