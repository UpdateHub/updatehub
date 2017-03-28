package tarball

import (
	"fmt"
	"path"
	"testing"

	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/libarchive"
	"github.com/UpdateHub/updatehub/testsmocks/copymock"
	"github.com/UpdateHub/updatehub/testsmocks/filesystemmock"
	"github.com/UpdateHub/updatehub/testsmocks/libarchivemock"
	"github.com/UpdateHub/updatehub/testsmocks/mtdmock"
	"github.com/UpdateHub/updatehub/testsmocks/ubifsmock"
	"github.com/UpdateHub/updatehub/utils"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestTarballInit(t *testing.T) {
	val, err := installmodes.GetObject("tarball")
	assert.NoError(t, err)

	tb1, ok := val.(*TarballObject)
	if !ok {
		t.Error("Failed to cast return value of \"installmodes.GetObject()\" to TarballObject")
	}

	osFs := afero.NewOsFs()
	tb2 := &TarballObject{
		FileSystemHelper:  &utils.FileSystem{},
		LibArchiveBackend: &libarchive.LibArchive{},
		FileSystemBackend: osFs,
		Copier:            &utils.ExtendedIO{},
		MtdUtils:          &utils.MtdUtilsImpl{},
		UbifsUtils: &utils.UbifsUtilsImpl{
			CmdLineExecuter: &utils.CmdLine{},
		},
	}

	assert.Equal(t, tb2, tb1)
}

func TestTarballSetupWithDeviceTargetType(t *testing.T) {
	memFs := afero.NewMemMapFs()

	targetDevice := "/dev/sdn5"

	tb := TarballObject{FileSystemBackend: memFs}
	tb.TargetType = "device"
	tb.Target = targetDevice

	err := tb.Setup()

	assert.NoError(t, err)
	assert.Equal(t, targetDevice, tb.targetDevice)
}

func TestTarballSetupWithMtdnameTargetType(t *testing.T) {
	memFs := afero.NewMemMapFs()

	mtdname := "system0"
	targetDevice := "/dev/mtd5"

	mum := &mtdmock.MtdUtilsMock{}
	mum.On("GetTargetDeviceFromMtdName", memFs, mtdname).Return(targetDevice, nil)

	tb := TarballObject{FileSystemBackend: memFs, MtdUtils: mum}
	tb.TargetType = "mtdname"
	tb.Target = mtdname

	err := tb.Setup()

	assert.NoError(t, err)
	assert.Equal(t, targetDevice, tb.targetDevice)

	mum.AssertExpectations(t)
}

func TestTarballSetupWithMtdnameTargetTypeWithError(t *testing.T) {
	memFs := afero.NewMemMapFs()

	mtdname := "system0"

	mum := &mtdmock.MtdUtilsMock{}
	mum.On("GetTargetDeviceFromMtdName", memFs, mtdname).Return("", fmt.Errorf("some error"))

	tb := TarballObject{FileSystemBackend: memFs, MtdUtils: mum}
	tb.TargetType = "mtdname"
	tb.Target = mtdname

	err := tb.Setup()

	assert.EqualError(t, err, "some error")
	assert.Equal(t, "", tb.targetDevice)

	mum.AssertExpectations(t)
}

func TestTarballSetupWithUbivolumeTargetType(t *testing.T) {
	memFs := afero.NewMemMapFs()

	ubivolume := "system0"
	targetDevice := "/dev/ubi5_6"

	uum := &ubifsmock.UbifsUtilsMock{}
	uum.On("GetTargetDeviceFromUbiVolumeName", memFs, ubivolume).Return(targetDevice, nil)

	tb := TarballObject{FileSystemBackend: memFs, UbifsUtils: uum}
	tb.TargetType = "ubivolume"
	tb.Target = ubivolume

	err := tb.Setup()

	assert.NoError(t, err)
	assert.Equal(t, targetDevice, tb.targetDevice)

	uum.AssertExpectations(t)
}

func TestTarballSetupWithUbivolumeTargetTypeWithError(t *testing.T) {
	memFs := afero.NewMemMapFs()

	ubivolume := "system0"

	uum := &ubifsmock.UbifsUtilsMock{}
	uum.On("GetTargetDeviceFromUbiVolumeName", memFs, ubivolume).Return("", fmt.Errorf("some error"))

	tb := TarballObject{FileSystemBackend: memFs, UbifsUtils: uum}
	tb.TargetType = "ubivolume"
	tb.Target = ubivolume

	err := tb.Setup()

	assert.EqualError(t, err, "some error")
	assert.Equal(t, "", tb.targetDevice)

	uum.AssertExpectations(t)
}

func TestTarballSetupWithNotSupportedTargetTypes(t *testing.T) {
	tb := TarballObject{}

	tb.TargetType = "invalid"
	err := tb.Setup()
	assert.EqualError(t, err, "target-type 'invalid' is not supported for the 'tarball' handler. Its value must be one of: 'device', 'ubivolume' or 'mtdname'")
}

func TestTarballInstallWithFormatError(t *testing.T) {
	memFs := afero.NewMemMapFs()
	lam := &libarchivemock.LibArchiveMock{}
	cm := &copymock.CopierMock{}

	targetDevice := "/dev/xx1"
	fsType := "ext4"
	formatOptions := "-y"

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("Format", targetDevice, fsType, formatOptions).Return(fmt.Errorf("format error"))
	tb := TarballObject{
		FileSystemHelper:  fsm,
		Copier:            cm,
		FileSystemBackend: memFs,
		LibArchiveBackend: lam,
	}
	tb.MustFormat = true
	tb.Target = targetDevice
	tb.TargetType = "device"
	tb.FSType = fsType
	tb.FormatOptions = formatOptions

	err := tb.Install()

	assert.EqualError(t, err, "format error")
	fsm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)
}

func TestTarballInstallWithTempDirError(t *testing.T) {
	memFs := afero.NewMemMapFs()
	lam := &libarchivemock.LibArchiveMock{}
	cm := &copymock.CopierMock{}

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("TempDir", "tarball-handler").Return("", fmt.Errorf("temp dir error"))
	tb := TarballObject{
		FileSystemHelper:  fsm,
		Copier:            cm,
		FileSystemBackend: memFs,
		LibArchiveBackend: lam,
	}

	err := tb.Install()

	assert.EqualError(t, err, "temp dir error")
	fsm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)
}

func TestTarballInstallWithMountError(t *testing.T) {
	memFs := afero.NewMemMapFs()
	lam := &libarchivemock.LibArchiveMock{}
	cm := &copymock.CopierMock{}

	tempDirPath, err := afero.TempDir(memFs, "", "tarball-handler")
	assert.NoError(t, err)

	targetDevice := "/dev/xx1"
	fsType := "ext4"
	mountOptions := "-o rw"

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("TempDir", "tarball-handler").Return(tempDirPath, nil)
	fsm.On("Mount", targetDevice, tempDirPath, fsType, mountOptions).Return(fmt.Errorf("mount error"))
	tb := TarballObject{
		FileSystemHelper:  fsm,
		Copier:            cm,
		FileSystemBackend: memFs,
		LibArchiveBackend: lam,
	}
	tb.Target = targetDevice
	tb.FSType = fsType
	tb.MountOptions = mountOptions

	err = tb.Install()

	assert.EqualError(t, err, "mount error")
	fsm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)

	tempDirExists, err := afero.Exists(memFs, tempDirPath)
	assert.False(t, tempDirExists)
	assert.NoError(t, err)
}

func TestTarballInstallWithExtractError(t *testing.T) {
	memFs := afero.NewMemMapFs()

	tempDirPath, err := afero.TempDir(memFs, "", "tarball-handler")
	assert.NoError(t, err)

	targetDevice := "/dev/xx1"
	targetPath := "/inner-path"
	fsType := "ext4"
	mountOptions := "-o rw"
	sha256sum := "b5f11b9a8090325b79bc9222d5e8ccc084427aa1d2a2532d80a59ecca2ca6f4e"
	compressed := true

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("TempDir", "tarball-handler").Return(tempDirPath, nil)
	fsm.On("Mount", targetDevice, tempDirPath, fsType, mountOptions).Return(nil)
	fsm.On("Umount", tempDirPath).Return(nil)

	cm := &copymock.CopierMock{}

	lam := &libarchivemock.LibArchiveMock{}
	lam.On("Unpack", sha256sum, path.Join(tempDirPath, targetPath), false).Return(fmt.Errorf("unpack error"))

	tb := TarballObject{
		FileSystemHelper:  fsm,
		Copier:            cm,
		FileSystemBackend: memFs,
		LibArchiveBackend: lam,
	}

	tb.Target = targetDevice
	tb.TargetPath = targetPath
	tb.FSType = fsType
	tb.MountOptions = mountOptions
	tb.Sha256sum = sha256sum
	tb.Compressed = compressed

	err = tb.Install()

	assert.EqualError(t, err, "unpack error")
	fsm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)

	tempDirExists, err := afero.Exists(memFs, tempDirPath)
	assert.False(t, tempDirExists)
	assert.NoError(t, err)
}

func TestTarballInstallWithUmountError(t *testing.T) {
	memFs := afero.NewMemMapFs()

	tempDirPath, err := afero.TempDir(memFs, "", "tarball-handler")
	assert.NoError(t, err)

	targetDevice := "/dev/xx1"
	targetPath := "/inner-path"
	fsType := "ext4"
	mountOptions := "-o rw"
	sha256sum := "b5f11b9a8090325b79bc9222d5e8ccc084427aa1d2a2532d80a59ecca2ca6f4e"
	compressed := true

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("TempDir", "tarball-handler").Return(tempDirPath, nil)
	fsm.On("Mount", targetDevice, tempDirPath, fsType, mountOptions).Return(nil)
	fsm.On("Umount", tempDirPath).Return(fmt.Errorf("umount error"))

	cm := &copymock.CopierMock{}

	lam := &libarchivemock.LibArchiveMock{}
	lam.On("Unpack", sha256sum, path.Join(tempDirPath, targetPath), false).Return(nil)

	tb := TarballObject{
		FileSystemHelper:  fsm,
		Copier:            cm,
		FileSystemBackend: memFs,
		LibArchiveBackend: lam,
	}

	tb.Target = targetDevice
	tb.TargetPath = targetPath
	tb.FSType = fsType
	tb.MountOptions = mountOptions
	tb.Sha256sum = sha256sum
	tb.Compressed = compressed

	err = tb.Install()

	assert.EqualError(t, err, "umount error")
	fsm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)

	tempDirExists, err := afero.Exists(memFs, tempDirPath)
	assert.True(t, tempDirExists)
	assert.NoError(t, err)
}

func TestTarballInstallWithUnpackANDUmountErrors(t *testing.T) {
	memFs := afero.NewMemMapFs()

	tempDirPath, err := afero.TempDir(memFs, "", "tarball-handler")
	assert.NoError(t, err)

	targetDevice := "/dev/xx1"
	targetPath := "/inner-path"
	fsType := "ext4"
	mountOptions := "-o rw"
	sha256sum := "b5f11b9a8090325b79bc9222d5e8ccc084427aa1d2a2532d80a59ecca2ca6f4e"
	compressed := true

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("TempDir", "tarball-handler").Return(tempDirPath, nil)
	fsm.On("Mount", targetDevice, tempDirPath, fsType, mountOptions).Return(nil)
	fsm.On("Umount", tempDirPath).Return(fmt.Errorf("umount error"))

	cm := &copymock.CopierMock{}

	lam := &libarchivemock.LibArchiveMock{}
	lam.On("Unpack", sha256sum, path.Join(tempDirPath, targetPath), false).Return(fmt.Errorf("unpack error"))

	tb := TarballObject{
		FileSystemHelper:  fsm,
		Copier:            cm,
		FileSystemBackend: memFs,
		LibArchiveBackend: lam,
	}

	tb.Target = targetDevice
	tb.TargetPath = targetPath
	tb.FSType = fsType
	tb.MountOptions = mountOptions
	tb.Sha256sum = sha256sum
	tb.Compressed = compressed

	err = tb.Install()

	assert.EqualError(t, err, "(unpack error); (umount error)")
	fsm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)

	tempDirExists, err := afero.Exists(memFs, tempDirPath)
	assert.True(t, tempDirExists)
	assert.NoError(t, err)
}

func TestTarballInstallWithSuccess(t *testing.T) {
	memFs := afero.NewMemMapFs()

	tempDirPath, err := afero.TempDir(memFs, "", "tarball-handler")
	assert.NoError(t, err)

	targetDevice := "/dev/xx1"
	targetPath := "/inner-path"
	fsType := "ext4"
	mountOptions := "-o rw"
	sha256sum := "b5f11b9a8090325b79bc9222d5e8ccc084427aa1d2a2532d80a59ecca2ca6f4e"
	compressed := true

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("TempDir", "tarball-handler").Return(tempDirPath, nil)
	fsm.On("Mount", targetDevice, tempDirPath, fsType, mountOptions).Return(nil)
	fsm.On("Umount", tempDirPath).Return(nil)

	cm := &copymock.CopierMock{}

	lam := &libarchivemock.LibArchiveMock{}
	lam.On("Unpack", sha256sum, path.Join(tempDirPath, targetPath), false).Return(nil)

	tb := TarballObject{
		FileSystemHelper:  fsm,
		Copier:            cm,
		FileSystemBackend: memFs,
		LibArchiveBackend: lam,
	}

	tb.Target = targetDevice
	tb.TargetPath = targetPath
	tb.FSType = fsType
	tb.MountOptions = mountOptions
	tb.Sha256sum = sha256sum
	tb.Compressed = compressed

	err = tb.Install()

	assert.NoError(t, err)
	fsm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)

	tempDirExists, err := afero.Exists(memFs, tempDirPath)
	assert.False(t, tempDirExists)
	assert.NoError(t, err)
}

func TestTarballCleanupNil(t *testing.T) {
	tb := TarballObject{}
	assert.Nil(t, tb.Cleanup())
}
