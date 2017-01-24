package copy

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"bitbucket.org/ossystems/agent/installmodes"
	"bitbucket.org/ossystems/agent/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func dirExists(path string) (bool, error) {
	_, err := os.Stat(path)

	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return true, err
}

func TestCopyInit(t *testing.T) {
	val, err := installmodes.GetObject("copy")
	assert.NoError(t, err)

	cp1, ok := val.(*CopyObject)
	if !ok {
		t.Error("Failed to cast return value of \"installmodes.GetObject()\" to CopyObject")
	}

	cp2 := &CopyObject{FileSystemHelper: &utils.FileSystem{}, CustomCopier: &utils.CustomCopy{}}

	assert.Equal(t, cp2, cp1)
}

func TestCopySetupWithSuccess(t *testing.T) {
	cp := CopyObject{}
	cp.TargetType = "device"
	err := cp.Setup()
	assert.NoError(t, err)
}

func TestCopySetupWithNotSupportedTargetTypes(t *testing.T) {
	cp := CopyObject{}

	cp.TargetType = "ubivolume"
	err := cp.Setup()
	assert.EqualError(t, err, "target-type 'ubivolume' is not supported for the 'copy' handler. Its value must be 'device'")

	cp.TargetType = "mtdname"
	err = cp.Setup()
	assert.EqualError(t, err, "target-type 'mtdname' is not supported for the 'copy' handler. Its value must be 'device'")

	cp.TargetType = "someother"
	err = cp.Setup()
	assert.EqualError(t, err, "target-type 'someother' is not supported for the 'copy' handler. Its value must be 'device'")
}

type FileSystemHelperMock struct {
	*mock.Mock
}

func (fsm FileSystemHelperMock) Format(targetDevice string, fsType string, formatOptions string) error {
	args := fsm.Called(targetDevice, fsType, formatOptions)
	return args.Error(0)
}

func (fsm FileSystemHelperMock) Mount(targetDevice string, mountPath string, fsType string, mountOptions string) error {
	args := fsm.Called(targetDevice, mountPath, fsType, mountOptions)
	return args.Error(0)
}

func (fsm FileSystemHelperMock) Umount(mountPath string) error {
	args := fsm.Called(mountPath)
	return args.Error(0)
}

func (fsm FileSystemHelperMock) TempDir(prefix string) (string, error) {
	args := fsm.Called(prefix)
	return args.String(0), args.Error(1)
}

func TestCopyInstallWithFormatError(t *testing.T) {
	targetDevice := "/dev/xx1"
	fsType := "ext4"
	formatOptions := "-y"

	fsm := FileSystemHelperMock{&mock.Mock{}}
	fsm.On("Format", targetDevice, fsType, formatOptions).Return(fmt.Errorf("format error"))
	cp := CopyObject{FileSystemHelper: fsm}
	cp.MustFormat = true
	cp.Target = targetDevice
	cp.FSType = fsType
	cp.FormatOptions = formatOptions

	err := cp.Install()

	assert.EqualError(t, err, "format error")
	fsm.AssertExpectations(t)
}

func TestCopyInstallWithTempDirError(t *testing.T) {
	fsm := FileSystemHelperMock{&mock.Mock{}}
	fsm.On("TempDir", "copy-handler").Return("", fmt.Errorf("temp dir error"))
	cp := CopyObject{FileSystemHelper: fsm}

	err := cp.Install()

	assert.EqualError(t, err, "temp dir error")
	fsm.AssertExpectations(t)
}

func TestCopyInstallWithMountError(t *testing.T) {
	tempDirPath, err := ioutil.TempDir("", "copy-handler")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDirPath)

	targetDevice := "/dev/xx1"
	fsType := "ext4"
	mountOptions := "-o rw"

	fsm := FileSystemHelperMock{&mock.Mock{}}
	fsm.On("TempDir", "copy-handler").Return(tempDirPath, nil)
	fsm.On("Mount", targetDevice, tempDirPath, fsType, mountOptions).Return(fmt.Errorf("mount error"))
	cp := CopyObject{FileSystemHelper: fsm}
	cp.Target = targetDevice
	cp.FSType = fsType
	cp.MountOptions = mountOptions

	err = cp.Install()

	assert.EqualError(t, err, "mount error")
	fsm.AssertExpectations(t)

	tempDirExists, err := dirExists(tempDirPath)
	assert.False(t, tempDirExists)
	assert.NoError(t, err)
}

type CustomCopierMock struct {
	*mock.Mock
}

func (ccm CustomCopierMock) CopyFile(sourcePath string, targetPath string, chunkSize int, skip int, seek int, count int, truncate bool, compressed bool) error {
	args := ccm.Called(sourcePath, targetPath, chunkSize, skip, seek, count, truncate, compressed)
	return args.Error(0)
}

func TestCopyInstallWithCopyFileError(t *testing.T) {
	tempDirPath, err := ioutil.TempDir("", "copy-handler")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDirPath)

	targetDevice := "/dev/xx1"
	targetPath := "/inner-path"
	fsType := "ext4"
	mountOptions := "-o rw"
	sha256sum := "2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226"
	compressed := false

	fsm := FileSystemHelperMock{&mock.Mock{}}
	fsm.On("TempDir", "copy-handler").Return(tempDirPath, nil)
	fsm.On("Mount", targetDevice, tempDirPath, fsType, mountOptions).Return(nil)
	fsm.On("Umount", tempDirPath).Return(nil)

	cc := CustomCopierMock{&mock.Mock{}}
	cc.On("CopyFile", sha256sum, path.Join(tempDirPath, targetPath), 128*1024, 0, 0, -1, true, compressed).Return(fmt.Errorf("copy file error"))

	cp := CopyObject{FileSystemHelper: fsm, CustomCopier: cc}
	cp.Target = targetDevice
	cp.TargetPath = targetPath
	cp.FSType = fsType
	cp.MountOptions = mountOptions
	cp.Sha256sum = sha256sum
	cp.Compressed = compressed

	err = cp.Install()

	assert.EqualError(t, err, "copy file error")
	fsm.AssertExpectations(t)
	cc.AssertExpectations(t)

	tempDirExists, err := dirExists(tempDirPath)
	assert.False(t, tempDirExists)
	assert.NoError(t, err)
}

func TestCopyInstallWithUmountError(t *testing.T) {
	tempDirPath, err := ioutil.TempDir("", "copy-handler")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDirPath)

	targetDevice := "/dev/xx1"
	targetPath := "/inner-path"
	fsType := "ext4"
	mountOptions := "-o rw"
	sha256sum := "2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226"
	compressed := false

	fsm := FileSystemHelperMock{&mock.Mock{}}
	fsm.On("TempDir", "copy-handler").Return(tempDirPath, nil)
	fsm.On("Mount", targetDevice, tempDirPath, fsType, mountOptions).Return(nil)
	fsm.On("Umount", tempDirPath).Return(fmt.Errorf("umount error"))

	cc := CustomCopierMock{&mock.Mock{}}
	cc.On("CopyFile", sha256sum, path.Join(tempDirPath, targetPath), 128*1024, 0, 0, -1, true, compressed).Return(nil)

	cp := CopyObject{FileSystemHelper: fsm, CustomCopier: cc}
	cp.Target = targetDevice
	cp.TargetPath = targetPath
	cp.FSType = fsType
	cp.MountOptions = mountOptions
	cp.Sha256sum = sha256sum
	cp.Compressed = compressed

	err = cp.Install()

	assert.EqualError(t, err, "umount error")
	fsm.AssertExpectations(t)
	cc.AssertExpectations(t)

	tempDirExists, err := dirExists(tempDirPath)
	assert.True(t, tempDirExists)
	assert.NoError(t, err)
}

func TestCopyInstallWithCopyFileANDUmountErrors(t *testing.T) {
	tempDirPath, err := ioutil.TempDir("", "copy-handler")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDirPath)

	targetDevice := "/dev/xx1"
	targetPath := "/inner-path"
	fsType := "ext4"
	mountOptions := "-o rw"
	sha256sum := "2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226"
	compressed := false

	fsm := FileSystemHelperMock{&mock.Mock{}}
	fsm.On("TempDir", "copy-handler").Return(tempDirPath, nil)
	fsm.On("Mount", targetDevice, tempDirPath, fsType, mountOptions).Return(nil)
	fsm.On("Umount", tempDirPath).Return(fmt.Errorf("umount error"))

	cc := CustomCopierMock{&mock.Mock{}}
	cc.On("CopyFile", sha256sum, path.Join(tempDirPath, targetPath), 128*1024, 0, 0, -1, true, compressed).Return(fmt.Errorf("copy file error"))

	cp := CopyObject{FileSystemHelper: fsm, CustomCopier: cc}
	cp.Target = targetDevice
	cp.TargetPath = targetPath
	cp.FSType = fsType
	cp.MountOptions = mountOptions
	cp.Sha256sum = sha256sum
	cp.Compressed = compressed

	err = cp.Install()

	assert.EqualError(t, err, "(copy file error); (umount error)")
	fsm.AssertExpectations(t)
	cc.AssertExpectations(t)

	tempDirExists, err := dirExists(tempDirPath)
	assert.True(t, tempDirExists)
	assert.NoError(t, err)
}

func TestCopyInstallWithSuccess(t *testing.T) {
	testCases := []struct {
		Name              string
		Sha256sum         string
		Target            string
		TargetType        string
		TargetPath        string
		FSType            string
		FormatOptions     string
		MustFormat        bool
		MountOptions      string
		ChunkSize         int
		ExpectedChunkSize int
		Compressed        bool
	}{
		{
			"WithAllFields",
			"2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226",
			"/dev/xx1",
			"device",
			"/inner-path",
			"ext4",
			"-y",
			true,
			"-o rw",
			2048,
			2048,
			false,
		},
		{
			"WithNegativeChunkSize",
			"2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226",
			"/dev/xx1",
			"device",
			"/inner-path",
			"ext4",
			"-y",
			true,
			"-o rw",
			-1,
			128 * 1024,
			false,
		},
		{
			"WithChunkSizeZero",
			"2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226",
			"/dev/xx1",
			"device",
			"/inner-path",
			"ext4",
			"",
			false,
			"-o rw",
			0,
			128 * 1024,
			false,
		},
		{
			"WithCompressed",
			"2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226",
			"/dev/xx1",
			"device",
			"/inner-path",
			"ext4",
			"",
			false,
			"-o rw",
			2048,
			2048,
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			tempDirPath, err := ioutil.TempDir("", "copy-handler")
			assert.NoError(t, err)
			defer os.RemoveAll(tempDirPath)

			fsm := FileSystemHelperMock{&mock.Mock{}}
			if tc.MustFormat {
				fsm.On("Format", tc.Target, tc.FSType, tc.FormatOptions).Return(nil)
			}
			fsm.On("TempDir", "copy-handler").Return(tempDirPath, nil)
			fsm.On("Mount", tc.Target, tempDirPath, tc.FSType, tc.MountOptions).Return(nil)
			fsm.On("Umount", tempDirPath).Return(nil)

			cc := CustomCopierMock{&mock.Mock{}}
			cc.On("CopyFile", tc.Sha256sum, path.Join(tempDirPath, tc.TargetPath), tc.ExpectedChunkSize, 0, 0, -1, true, tc.Compressed).Return(nil)

			cp := CopyObject{FileSystemHelper: fsm, CustomCopier: cc}
			cp.Target = tc.Target
			cp.TargetType = tc.TargetType
			cp.TargetPath = tc.TargetPath
			cp.FSType = tc.FSType
			cp.MountOptions = tc.MountOptions
			cp.Sha256sum = tc.Sha256sum
			cp.FormatOptions = tc.FormatOptions
			cp.MustFormat = tc.MustFormat
			cp.ChunkSize = tc.ChunkSize
			cp.Compressed = tc.Compressed

			err = cp.Install()

			assert.NoError(t, err)
			fsm.AssertExpectations(t)
			cc.AssertExpectations(t)

			tempDirExists, err := dirExists(tempDirPath)
			assert.False(t, tempDirExists)
			assert.NoError(t, err)
		})
	}
}

func TestCopyCleanupNil(t *testing.T) {
	cp := CopyObject{}
	assert.Nil(t, cp.Cleanup())
}
