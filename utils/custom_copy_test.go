package utils

import (
	"fmt"
	"io"
	"os"
	"path"
	"syscall"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCustomCopyFileIntegration(t *testing.T) {
	testCases := []struct {
		Name              string
		SourceFileContent []byte
	}{
		{
			"Success",
			[]byte("content"),
		},
		{
			"ZeroBytesSourceFile",
			[]byte(""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			memFs := afero.NewMemMapFs()

			testPath, err := afero.TempDir(memFs, "", "CustomCopyFile-test")
			assert.NoError(t, err)

			sourcePath := path.Join(testPath, "source.txt")
			source, err := memFs.Create(sourcePath)
			assert.Nil(t, err)
			_, err = source.Write(tc.SourceFileContent)
			assert.Nil(t, err)
			err = source.Close()
			assert.Nil(t, err)

			targetPath := path.Join(testPath, "target.txt")

			chunkSize := 128
			skip := 0
			seek := 0
			count := -1
			truncate := true
			compressed := false

			pathExists, err := afero.Exists(memFs, targetPath)
			assert.False(t, pathExists)
			assert.NoError(t, err)

			cc := CustomCopy{FileSystemBackend: memFs}
			err = cc.CopyFile(sourcePath, targetPath, chunkSize,
				skip, seek, count, truncate, compressed)
			assert.NoError(t, err)

			pathExists, err = afero.Exists(memFs, targetPath)
			assert.True(t, pathExists)
			assert.NoError(t, err)

			data, err := afero.ReadFile(memFs, targetPath)
			assert.NoError(t, err)
			assert.Equal(t, tc.SourceFileContent, data)
		})
	}
}

func TestCustomCopyFileWithSuccessUsingMocks(t *testing.T) {
	const (
		chunkSize = 128 * 1024
	)

	sourceMock := FileMock{&mock.Mock{}}
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

	targetMock := FileMock{&mock.Mock{}}
	targetContent := []uint8("")
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		targetContent = arg
	}).Return(len(targetContent), nil).Once()
	targetMock.On("Close").Return(nil)

	fom := FileSystemBackendMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	cc := CustomCopy{FileSystemBackend: fom}

	err := cc.CopyFile("source.txt", "target.txt", chunkSize,
		0, 0, -1, true, false)
	assert.NoError(t, err)

	assert.Equal(t, sourceContent, targetContent)

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCustomCopyFileWithSuccessWithMultipleChunksUsingMocks(t *testing.T) {
	const (
		chunkSize = 2
	)

	sourceMock := FileMock{&mock.Mock{}}
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

	targetMock := FileMock{&mock.Mock{}}
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

	fom := FileSystemBackendMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	cc := CustomCopy{FileSystemBackend: fom}

	err := cc.CopyFile("source.txt", "target.txt", chunkSize,
		0, 0, -1, true, false)
	assert.NoError(t, err)

	assert.Equal(t, sourceContent, targetContent)

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCustomCopyFileWithSuccessUsingSkipAndSeekWithMocks(t *testing.T) {
	const (
		skip      = 3
		seek      = 1
		chunkSize = 4
	)

	sourceMock := FileMock{&mock.Mock{}}
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

	targetMock := FileMock{&mock.Mock{}}
	writeContent := []uint8("")
	targetMock.On("Seek", int64(seek*chunkSize), io.SeekStart).Return(int64(skip*chunkSize), nil)
	targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		writeContent = arg
	}).Return(len(writeContent), nil).Once()
	targetMock.On("Close").Return(nil)

	fom := FileSystemBackendMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	cc := CustomCopy{FileSystemBackend: fom}

	err := cc.CopyFile("source.txt", "target.txt", chunkSize,
		skip, seek, -1, true, false)
	assert.NoError(t, err)

	assert.Equal(t, readContent, writeContent)

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCustomCopyFileUsingCountWithMocks(t *testing.T) {
	const (
		chunkSize = 1
		count     = 3
	)

	sourceMock := FileMock{&mock.Mock{}}
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

	targetMock := FileMock{&mock.Mock{}}
	targetContent := []uint8("")
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)

	for i := 0; i < count; i++ {
		targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
			arg := args.Get(0).([]uint8)
			targetContent = append(targetContent, arg...)
		}).Return(chunkSize, nil).Once()
	}

	targetMock.On("Close").Return(nil)

	fom := FileSystemBackendMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	cc := CustomCopy{FileSystemBackend: fom}

	err := cc.CopyFile("source.txt", "target.txt", chunkSize,
		0, 0, count, true, false)
	assert.NoError(t, err)

	assert.Equal(t, sourceContent[:count*chunkSize], targetContent)

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCustomCopyFileUsingNegativeCountWithMocks(t *testing.T) {
	// negative count means the whole file
	const (
		chunkSize = 1
		count     = -1
	)

	sourceMock := FileMock{&mock.Mock{}}
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

	targetMock := FileMock{&mock.Mock{}}
	targetContent := []uint8("")
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)

	for i := 0; i < len(sourceContent); i++ {
		targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
			arg := args.Get(0).([]uint8)
			targetContent = append(targetContent, arg...)
		}).Return(chunkSize, nil).Once()
	}

	targetMock.On("Close").Return(nil)

	fom := FileSystemBackendMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	cc := CustomCopy{FileSystemBackend: fom}

	err := cc.CopyFile("source.txt", "target.txt", chunkSize,
		0, 0, count, true, false)
	assert.NoError(t, err)

	assert.Equal(t, sourceContent, targetContent)

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCustomCopyFileNotUsingTruncateWithMocks(t *testing.T) {
	const (
		truncate = false
	)

	sourceMock := FileMock{&mock.Mock{}}
	sourceContent := []uint8("test")
	sourceMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		copy(arg, sourceContent)
	}).Return(len(sourceContent), nil).Once()
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Return(0, io.EOF).Once()
	sourceMock.On("Close").Return(nil)

	targetMock := FileMock{&mock.Mock{}}
	targetContent := []uint8("")
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		targetContent = append(targetContent, arg...)
	}).Return(len(targetContent), nil).Once()
	targetMock.On("Close").Return(nil)

	fom := FileSystemBackendMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE, os.FileMode(0666)).Return(targetMock, nil)

	cc := CustomCopy{FileSystemBackend: fom}

	err := cc.CopyFile("source.txt", "target.txt", ChunkSize,
		0, 0, -1, truncate, false)
	assert.NoError(t, err)

	assert.Equal(t, sourceContent, targetContent)

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCustomCopyFileWithOpenError(t *testing.T) {
	targetMock := FileMock{&mock.Mock{}}

	pathError := &os.PathError{
		Op:   "open",
		Path: "source.txt",
		Err:  syscall.ENOSPC,
	}

	fom := FileSystemBackendMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return((*FileMock)(nil), pathError)

	cc := CustomCopy{FileSystemBackend: fom}

	err := cc.CopyFile("source.txt", "target.txt", ChunkSize,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "open source.txt: no space left on device")

	fom.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCustomCopyFileWithOpenFileError(t *testing.T) {
	sourceMock := FileMock{&mock.Mock{}}
	sourceMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	sourceMock.On("Close").Return(nil)

	pathError := &os.PathError{
		Op:   "open",
		Path: "target.txt",
		Err:  syscall.ENOSPC,
	}

	fom := FileSystemBackendMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return((*FileMock)(nil), pathError)

	cc := CustomCopy{FileSystemBackend: fom}

	err := cc.CopyFile("source.txt", "target.txt", ChunkSize,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "open target.txt: no space left on device")

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
}

func TestCustomCopyFileWithReadError(t *testing.T) {
	sourceMock := FileMock{&mock.Mock{}}
	sourceMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Return(0, io.ErrClosedPipe).Once()
	sourceMock.On("Close").Return(nil)

	targetMock := FileMock{&mock.Mock{}}
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Close").Return(nil)

	fom := FileSystemBackendMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	cc := CustomCopy{FileSystemBackend: fom}

	err := cc.CopyFile("source.txt", "target.txt", ChunkSize,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "io: read/write on closed pipe")

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCustomCopyFileWithWriteError(t *testing.T) {
	sourceMock := FileMock{&mock.Mock{}}
	sourceContent := []uint8("test")
	sourceMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		copy(arg, sourceContent)
	}).Return(len(sourceContent), nil).Once()
	sourceMock.On("Close").Return(nil)

	targetMock := FileMock{&mock.Mock{}}
	targetContent := []uint8("")
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		targetContent = arg
	}).Return(0, io.ErrClosedPipe).Once()
	targetMock.On("Close").Return(nil)

	fom := FileSystemBackendMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	cc := CustomCopy{FileSystemBackend: fom}

	err := cc.CopyFile("source.txt", "target.txt", ChunkSize,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "io: read/write on closed pipe")

	assert.Equal(t, sourceContent, targetContent)

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCustomCopyFileWithZeroedChunkSize(t *testing.T) {
	const (
		chunkSize = 0
	)

	sourceMock := FileMock{&mock.Mock{}}
	sourceMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	sourceMock.On("Close").Return(nil)

	targetMock := FileMock{&mock.Mock{}}
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Close").Return(nil)

	fom := FileSystemBackendMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	cc := CustomCopy{FileSystemBackend: fom}

	err := cc.CopyFile("source.txt", "target.txt", chunkSize,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "Copy error: chunkSize can't be less than 1")

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCustomCopyFileWithNegativeChunkSize(t *testing.T) {
	const (
		chunkSize = -1
	)

	sourceMock := FileMock{&mock.Mock{}}
	sourceMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	sourceMock.On("Close").Return(nil)

	targetMock := FileMock{&mock.Mock{}}
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	targetMock.On("Close").Return(nil)

	fom := FileSystemBackendMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	cc := CustomCopy{FileSystemBackend: fom}

	err := cc.CopyFile("source.txt", "target.txt", chunkSize,
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

	sourceMock := FileMock{&mock.Mock{}}
	sourceMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), fmt.Errorf("Seek: invalid whence"))
	sourceMock.On("Close").Return(nil)

	fom := FileSystemBackendMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return(sourceMock, nil)

	cc := CustomCopy{FileSystemBackend: fom}

	err := cc.CopyFile("source.txt", "target.txt", chunkSize,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "Seek: invalid whence")

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
}

func TestCustomCopyFileWithSeekError(t *testing.T) {
	const (
		chunkSize = 128 * 1024
	)

	sourceMock := FileMock{&mock.Mock{}}
	sourceMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), nil)
	sourceMock.On("Close").Return(nil)

	targetMock := FileMock{&mock.Mock{}}
	targetMock.On("Seek", int64(0), io.SeekStart).Return(int64(0), fmt.Errorf("Seek: invalid whence"))
	targetMock.On("Close").Return(nil)

	fom := FileSystemBackendMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("OpenFile", "target.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666)).Return(targetMock, nil)

	cc := CustomCopy{FileSystemBackend: fom}

	err := cc.CopyFile("source.txt", "target.txt", chunkSize,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "Seek: invalid whence")

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}
