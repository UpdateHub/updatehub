package ubifs

import (
	"fmt"
	"os/exec"

	"github.com/spf13/afero"

	"code.ossystems.com.br/updatehub/agent/installmodes"
	"code.ossystems.com.br/updatehub/agent/libarchive"
	"code.ossystems.com.br/updatehub/agent/metadata"
	"code.ossystems.com.br/updatehub/agent/utils"
)

func init() {
	installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "ubifs",
		CheckRequirements: checkRequirements,
		GetObject:         getObject,
	})
}

func checkRequirements() error {
	for _, binary := range []string{"ubiupdatevol", "ubinfo"} {
		_, err := exec.LookPath(binary)
		if err != nil {
			return err
		}
	}

	return nil
}

func getObject() interface{} {
	cle := &utils.CmdLine{}

	return &UbifsObject{
		CmdLineExecuter:   cle,
		Copier:            &utils.ExtendedIO{},
		LibArchiveBackend: &libarchive.LibArchive{},
		FileSystemBackend: afero.NewOsFs(),
		UbifsUtils: &utils.UbifsUtilsImpl{
			CmdLineExecuter: cle,
		},
	}
}

// UbifsObject encapsulates the "ubifs" handler data and functions
type UbifsObject struct {
	metadata.ObjectMetadata
	metadata.CompressedObject
	utils.CmdLineExecuter
	utils.UbifsUtils
	utils.Copier      `json:"-"`
	LibArchiveBackend libarchive.API `json:"-"`
	FileSystemBackend afero.Fs

	Target     string `json:"target"`
	TargetType string `json:"target-type"`
}

// Setup implementation for the "ubifs" handler
func (ufs *UbifsObject) Setup() error {
	if ufs.TargetType != "ubivolume" {
		return fmt.Errorf("target-type '%s' is not supported for the 'ubifs' handler. Its value must be 'ubivolume'", ufs.TargetType)
	}

	return nil
}

// Install implementation for the "ubifs" handler
func (ufs *UbifsObject) Install() error {
	targetDevice, err := ufs.GetTargetDeviceFromUbiVolumeName(ufs.FileSystemBackend, ufs.Target)
	if err != nil {
		return err
	}

	// FIXME: for srcPath we need to: path.Join(ufs.UpdateDir, ufs.Sha256sum)
	srcPath := ufs.Sha256sum

	if ufs.Compressed {
		cmdline := fmt.Sprintf("ubiupdatevol -s %.0f %s -", ufs.UncompressedSize, targetDevice)
		copyErr := ufs.Copier.CopyToProcessStdin(ufs.FileSystemBackend, ufs.LibArchiveBackend, srcPath, cmdline, ufs.Compressed)
		err = copyErr
	} else {
		_, execErr := ufs.Execute(fmt.Sprintf("ubiupdatevol %s %s", targetDevice, srcPath))
		err = execErr
	}

	return err
}

// Cleanup implementation for the "ubifs" handler
func (ufs *UbifsObject) Cleanup() error {
	return nil
}
