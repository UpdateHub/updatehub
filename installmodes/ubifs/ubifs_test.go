package ubifs

import (
	"fmt"
	"os"
	"path"
	"testing"

	"bitbucket.org/ossystems/agent/installmodes"
	"bitbucket.org/ossystems/agent/installmodes/internal/testsutils"
	"bitbucket.org/ossystems/agent/testsmocks"
	"bitbucket.org/ossystems/agent/utils"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

const ubinfoStdoutTemplate string = `Volume ID:   %d (on ubi%d)
Type:        dynamic
Alignment:   1
Size:        407 LEBs (52512768 bytes, 50.1 MiB)
State:       OK
Name:        %s
Character device major/minor: 247:1`

func TestUbifsHelperImplWithASingleDeviceNode(t *testing.T) {
	ubivolume := "system0"
	deviceNumber := 1
	volumeID := 2

	memFs := afero.NewMemMapFs()
	memFs.MkdirAll("/dev", 0755)
	afero.WriteFile(memFs, fmt.Sprintf("/dev/ubi%d", deviceNumber), []byte("ubi_content"), 0755)

	clm := &testsmocks.CmdLineExecuterMock{}
	clm.On("Execute", fmt.Sprintf("ubinfo -d %d -N %s", deviceNumber, ubivolume)).Return([]byte(fmt.Sprintf(ubinfoStdoutTemplate, volumeID, deviceNumber, ubivolume)), nil)

	uhi := &UbifsHelperImpl{CmdLineExecuter: clm, FileSystemBackend: memFs}
	targetDevice, err := uhi.GetTargetDeviceFromUbiVolumeName(ubivolume)

	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("/dev/ubi%d_%d", deviceNumber, volumeID), targetDevice)

	clm.AssertExpectations(t)
}

func TestUbifsHelperImplWithMultipleUbiDeviceNodes(t *testing.T) {
	ubivolume := "system0"
	deviceNumber := 1
	volumeID := 2

	memFs := afero.NewMemMapFs()
	memFs.MkdirAll("/dev", 0755)
	afero.WriteFile(memFs, fmt.Sprintf("/dev/ubi%d", deviceNumber-1), []byte("ubi_content"), 0755)
	afero.WriteFile(memFs, fmt.Sprintf("/dev/ubi%d", deviceNumber), []byte("ubi_content"), 0755)
	afero.WriteFile(memFs, fmt.Sprintf("/dev/ubi%d", deviceNumber+1), []byte("ubi_content"), 0755)

	clm := &testsmocks.CmdLineExecuterMock{}
	clm.On("Execute", fmt.Sprintf("ubinfo -d %d -N %s", deviceNumber-1, ubivolume)).Return([]byte(""), fmt.Errorf("Error executing command"))
	clm.On("Execute", fmt.Sprintf("ubinfo -d %d -N %s", deviceNumber, ubivolume)).Return([]byte(fmt.Sprintf(ubinfoStdoutTemplate, volumeID, deviceNumber, ubivolume)), nil)

	uhi := &UbifsHelperImpl{CmdLineExecuter: clm, FileSystemBackend: memFs}
	targetDevice, err := uhi.GetTargetDeviceFromUbiVolumeName(ubivolume)

	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("/dev/ubi%d_%d", deviceNumber, volumeID), targetDevice)

	clm.AssertExpectations(t)
}

func TestUbifsHelperImplWithReadDirFailure(t *testing.T) {
	ubivolume := "system0"

	memFs := afero.NewMemMapFs()
	memFs.RemoveAll("/dev")

	clm := &testsmocks.CmdLineExecuterMock{}

	uhi := &UbifsHelperImpl{CmdLineExecuter: clm, FileSystemBackend: memFs}
	targetDevice, err := uhi.GetTargetDeviceFromUbiVolumeName(ubivolume)

	assert.EqualError(t, err, "open /dev: file does not exist")
	assert.Equal(t, "", targetDevice)

	clm.AssertExpectations(t)
}

func TestUbifsInit(t *testing.T) {
	val, err := installmodes.GetObject("ubifs")
	assert.NoError(t, err)

	f1, ok := val.(*UbifsObject)
	if !ok {
		t.Error("Failed to cast return value of \"installmodes.GetObject()\" to UbifsObject")
	}

	f2, ok := getObject().(*UbifsObject)
	if !ok {
		t.Error("Failed to cast return value of \"getObject()\" to UbifsObject")
	}

	assert.Equal(t, f2, f1)
}

func TestUbifsGetObject(t *testing.T) {
	f, ok := getObject().(*UbifsObject)

	if !ok {
		t.Error("Failed to cast return value of \"getObject()\" to UbifsObject")
	}

	cmd := f.CmdLineExecuter
	_, ok = cmd.(*utils.CmdLine)

	if !ok {
		t.Error("Failed to cast default implementation of \"CmdLineExecuter\" to CmdLine")
	}
}

func TestUbifsCheckRequirementsWithBinariesNotFound(t *testing.T) {
	testCases := []struct {
		Name   string
		Binary string
	}{
		{
			"UbiUpdateVolNotFound",
			"ubiupdatevol",
		},
		{
			"UbInfoNotFound",
			"ubinfo",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// setup a temp dir on PATH
			testPath := testsutils.SetupCheckRequirementsDir(t, []string{"ubiupdatevol", "ubinfo"})

			defer os.RemoveAll(testPath)
			err := os.Setenv("PATH", testPath)
			assert.NoError(t, err)

			// remove binary
			os.Remove(path.Join(testPath, tc.Binary))

			// test the call
			err = checkRequirements()

			assert.EqualError(t, err, fmt.Sprintf("exec: \"%s\": executable file not found in $PATH", tc.Binary))
		})
	}
}

func TestUbifsCheckRequirementsWithBinariesFound(t *testing.T) {
	// setup a temp dir on PATH
	testPath := testsutils.SetupCheckRequirementsDir(t, []string{"ubiupdatevol", "ubinfo"})
	defer os.RemoveAll(testPath)
	err := os.Setenv("PATH", testPath)
	assert.NoError(t, err)

	// test the call
	err = checkRequirements()

	assert.NoError(t, err)
}

func TestUbifsSetupWithUbivolumeTargetType(t *testing.T) {
	ufs := UbifsObject{}
	ufs.TargetType = "ubivolume"
	ufs.Target = "system0"
	err := ufs.Setup()
	assert.NoError(t, err)
}

func TestUbifsSetupWithNotSupportedTargetTypes(t *testing.T) {
	clm := &testsmocks.CmdLineExecuterMock{}

	ufs := UbifsObject{CmdLineExecuter: clm}

	ufs.TargetType = "unknown-type"
	err := ufs.Setup()
	assert.EqualError(t, err, "target-type 'unknown-type' is not supported for the 'ubifs' handler. Its value must be 'ubivolume'")

	ufs.TargetType = "mtdname"
	err = ufs.Setup()
	assert.EqualError(t, err, "target-type 'mtdname' is not supported for the 'ubifs' handler. Its value must be 'ubivolume'")

	clm.AssertExpectations(t)
}

func TestUbifsCleanupNil(t *testing.T) {
	ufs := UbifsObject{}
	assert.Nil(t, ufs.Cleanup())
}

func TestUbifsInstallWithSuccessNonCompressed(t *testing.T) {
	ubivolume := "system0"
	compressed := false
	targetDevice := "/dev/mtd3"
	sha256sum := "71c88745e5a72067f94aae0ecec6d45af8b0f6e1a37ef695df0b56711e192b86"

	clm := &testsmocks.CmdLineExecuterMock{}
	clm.On("Execute", fmt.Sprintf("ubiupdatevol %s %s", targetDevice, sha256sum)).Return([]byte("combinedoutput"), nil)

	uhm := &testsmocks.UbifsHelperMock{}
	uhm.On("GetTargetDeviceFromUbiVolumeName", ubivolume).Return(targetDevice, nil)

	ufs := UbifsObject{CmdLineExecuter: clm, UbifsHelper: uhm}
	ufs.TargetType = "ubivolume"
	ufs.Target = ubivolume
	ufs.Sha256sum = sha256sum
	ufs.Compressed = compressed
	err := ufs.Install()
	assert.NoError(t, err)

	clm.AssertExpectations(t)
	uhm.AssertExpectations(t)
}

func TestUbifsInstallWithSuccessCompressed(t *testing.T) {
	ubivolume := "system0"
	compressed := true
	targetDevice := "/dev/mtd3"
	sha256sum := "71c88745e5a72067f94aae0ecec6d45af8b0f6e1a37ef695df0b56711e192b86"
	srcPath := sha256sum
	uncompressedSize := 12345678.0
	cmdline := fmt.Sprintf("ubiupdatevol -s %.0f %s -", uncompressedSize, targetDevice)

	clm := &testsmocks.CmdLineExecuterMock{}

	uhm := &testsmocks.UbifsHelperMock{}
	uhm.On("GetTargetDeviceFromUbiVolumeName", ubivolume).Return(targetDevice, nil)

	lam := &testsmocks.LibArchiveMock{}

	fsm := &testsmocks.FileSystemBackendMock{}

	cpm := &testsmocks.CopierMock{}
	cpm.On("CopyToProcessStdin", fsm, lam, srcPath, cmdline, compressed).Return(nil)

	ufs := UbifsObject{
		CmdLineExecuter:   clm,
		UbifsHelper:       uhm,
		LibArchiveBackend: lam,
		FileSystemBackend: fsm,
		Copier:            cpm,
	}
	ufs.TargetType = "ubivolume"
	ufs.Target = ubivolume
	ufs.Sha256sum = sha256sum
	ufs.Compressed = compressed
	ufs.UncompressedSize = uncompressedSize

	err := ufs.Install()
	assert.NoError(t, err)

	clm.AssertExpectations(t)
	uhm.AssertExpectations(t)
}

func TestUbifsInstallWithCopyToProcessStdinFailure(t *testing.T) {
	ubivolume := "system0"
	compressed := true
	targetDevice := "/dev/mtd3"
	sha256sum := "71c88745e5a72067f94aae0ecec6d45af8b0f6e1a37ef695df0b56711e192b86"
	srcPath := sha256sum
	uncompressedSize := 12345678.0
	cmdline := fmt.Sprintf("ubiupdatevol -s %.0f %s -", uncompressedSize, targetDevice)

	clm := &testsmocks.CmdLineExecuterMock{}

	uhm := &testsmocks.UbifsHelperMock{}
	uhm.On("GetTargetDeviceFromUbiVolumeName", ubivolume).Return(targetDevice, nil)

	lam := &testsmocks.LibArchiveMock{}

	fsm := &testsmocks.FileSystemBackendMock{}

	cpm := &testsmocks.CopierMock{}
	cpm.On("CopyToProcessStdin", fsm, lam, srcPath, cmdline, compressed).Return(fmt.Errorf("process error"))

	ufs := UbifsObject{
		CmdLineExecuter:   clm,
		UbifsHelper:       uhm,
		LibArchiveBackend: lam,
		FileSystemBackend: fsm,
		Copier:            cpm,
	}
	ufs.TargetType = "ubivolume"
	ufs.Target = ubivolume
	ufs.Sha256sum = sha256sum
	ufs.Compressed = compressed
	ufs.UncompressedSize = uncompressedSize

	err := ufs.Install()
	assert.EqualError(t, err, "process error")

	clm.AssertExpectations(t)
	uhm.AssertExpectations(t)
}

func TestUbifsInstallWithGetTargetDeviceFromUbiVolumeNameFailure(t *testing.T) {
	ubivolume := "system0"
	compressed := false
	sha256sum := "71c88745e5a72067f94aae0ecec6d45af8b0f6e1a37ef695df0b56711e192b86"

	clm := &testsmocks.CmdLineExecuterMock{}

	uhm := &testsmocks.UbifsHelperMock{}
	uhm.On("GetTargetDeviceFromUbiVolumeName", ubivolume).Return("", fmt.Errorf("UBI volume '%s' wasn't found", ubivolume))

	ufs := UbifsObject{CmdLineExecuter: clm, UbifsHelper: uhm}
	ufs.TargetType = "ubivolume"
	ufs.Target = ubivolume
	ufs.Sha256sum = sha256sum
	ufs.Compressed = compressed
	err := ufs.Install()
	assert.EqualError(t, err, fmt.Sprintf("UBI volume '%s' wasn't found", ubivolume))

	clm.AssertExpectations(t)
	uhm.AssertExpectations(t)
}

func TestUbifsInstallWithUbiUpdateVolFailure(t *testing.T) {
	ubivolume := "system0"
	compressed := false
	targetDevice := "/dev/mtd3"
	sha256sum := "71c88745e5a72067f94aae0ecec6d45af8b0f6e1a37ef695df0b56711e192b86"

	clm := &testsmocks.CmdLineExecuterMock{}
	clm.On("Execute", fmt.Sprintf("ubiupdatevol %s %s", targetDevice, sha256sum)).Return([]byte("error"), fmt.Errorf("Error executing command"))

	uhm := &testsmocks.UbifsHelperMock{}
	uhm.On("GetTargetDeviceFromUbiVolumeName", ubivolume).Return(targetDevice, nil)

	ufs := UbifsObject{CmdLineExecuter: clm, UbifsHelper: uhm}
	ufs.TargetType = "ubivolume"
	ufs.Target = ubivolume
	ufs.Sha256sum = sha256sum
	ufs.Compressed = compressed
	err := ufs.Install()
	assert.EqualError(t, err, "Error executing command")

	clm.AssertExpectations(t)
	uhm.AssertExpectations(t)
}
