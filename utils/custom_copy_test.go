package utils

import (
	"io"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type FileOperationsMock struct {
	*mock.Mock
}

func (fom FileOperationsMock) Open(name string) (FileInterface, error) {
	args := fom.Called(name)
	return args.Get(0).(FileInterface), args.Error(1)
}

func (fom FileOperationsMock) Create(name string) (FileInterface, error) {
	args := fom.Called(name)
	return args.Get(0).(FileInterface), args.Error(1)
}

type FileMock struct {
	*mock.Mock
}

func (fm FileMock) Read(p []byte) (n int, err error) {
	args := fm.Called(p)
	return args.Int(0), args.Error(1)
}

func (fm FileMock) ReadAt(b []byte, off int64) (n int, err error) {
	args := fm.Called(b, off)
	return args.Int(0), args.Error(1)
}

func (fm FileMock) Seek(offset int64, whence int) (ret int64, err error) {
	args := fm.Called(offset, whence)
	return args.Get(0).(int64), args.Error(1)
}

func (fm FileMock) Write(b []byte) (n int, err error) {
	args := fm.Called(b)
	return args.Int(0), args.Error(1)
}

func (fm FileMock) Close() error {
	args := fm.Called()
	return args.Error(0)
}

func TestCustomCopyFileWithSuccessUsingMocks(t *testing.T) {
	sourceMock := FileMock{&mock.Mock{}}
	sourceContent := []uint8("test")
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		copy(arg, sourceContent)
	}).Return(len(sourceContent), nil).Once()
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Return(0, io.EOF).Once()
	sourceMock.On("Close").Return(nil)

	targetMock := FileMock{&mock.Mock{}}
	targetContent := []uint8("")
	targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		targetContent = arg
	}).Return(len(targetContent), nil).Once()
	targetMock.On("Close").Return(nil)

	fom := FileOperationsMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("Create", "target.txt").Return(targetMock, nil)

	cc := CustomCopy{FileOperations: fom}

	err := cc.CopyFile("source.txt", "target.txt", 128*1024,
		0, 0, -1, true, false)
	assert.NoError(t, err)

	assert.Equal(t, sourceContent, targetContent)

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCustomCopyFileWithSuccessUsingMocks(t *testing.T) {
	sourceMock := FileMock{&mock.Mock{}}
	sourceContent := []uint8("test")
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		copy(arg, sourceContent)
	}).Return(len(sourceContent), nil).Once()
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Return(0, io.EOF).Once()
	sourceMock.On("Close").Return(nil)

	targetMock := FileMock{&mock.Mock{}}
	targetContent := []uint8("")
	targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		targetContent = arg
	}).Return(len(targetContent), nil).Once()
	targetMock.On("Close").Return(nil)

	fom := FileOperationsMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("Create", "target.txt").Return(targetMock, nil)

	cc := CustomCopy{FileOperations: fom}

	err := cc.CopyFile("source.txt", "target.txt", 128*1024,
		0, 0, -1, true, false)
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

	fom := FileOperationsMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return((*FileMock)(nil), pathError)

	cc := CustomCopy{FileOperations: fom}

	err := cc.CopyFile("source.txt", "target.txt", 128*1024,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "open source.txt: no space left on device")

	fom.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCustomCopyFileWithCreateError(t *testing.T) {
	sourceMock := FileMock{&mock.Mock{}}
	sourceMock.On("Close").Return(nil)

	pathError := &os.PathError{
		Op:   "open",
		Path: "target.txt",
		Err:  syscall.ENOSPC,
	}

	fom := FileOperationsMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("Create", "target.txt").Return((*FileMock)(nil), pathError)

	cc := CustomCopy{FileOperations: fom}

	err := cc.CopyFile("source.txt", "target.txt", 128*1024,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "open target.txt: no space left on device")

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
}

func TestCustomCopyFileWithReadError(t *testing.T) {
	sourceMock := FileMock{&mock.Mock{}}
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Return(0, io.ErrClosedPipe).Once()
	sourceMock.On("Close").Return(nil)

	targetMock := FileMock{&mock.Mock{}}
	targetMock.On("Close").Return(nil)

	fom := FileOperationsMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("Create", "target.txt").Return(targetMock, nil)

	cc := CustomCopy{FileOperations: fom}

	err := cc.CopyFile("source.txt", "target.txt", 128*1024,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "io: read/write on closed pipe")

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}

func TestCustomCopyFileWithWriteError(t *testing.T) {
	sourceMock := FileMock{&mock.Mock{}}
	sourceContent := []uint8("test")
	sourceMock.On("Read", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		copy(arg, sourceContent)
	}).Return(len(sourceContent), nil).Once()
	sourceMock.On("Close").Return(nil)

	targetMock := FileMock{&mock.Mock{}}
	targetContent := []uint8("")
	targetMock.On("Write", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		targetContent = arg
	}).Return(0, io.ErrClosedPipe).Once()
	targetMock.On("Close").Return(nil)

	fom := FileOperationsMock{&mock.Mock{}}
	fom.On("Open", "source.txt").Return(sourceMock, nil)
	fom.On("Create", "target.txt").Return(targetMock, nil)

	cc := CustomCopy{FileOperations: fom}

	err := cc.CopyFile("source.txt", "target.txt", 128*1024,
		0, 0, -1, true, false)
	assert.EqualError(t, err, "io: read/write on closed pipe")

	assert.Equal(t, sourceContent, targetContent)

	fom.AssertExpectations(t)
	sourceMock.AssertExpectations(t)
	targetMock.AssertExpectations(t)
}
