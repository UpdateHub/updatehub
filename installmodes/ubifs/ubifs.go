package ubifs

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/afero"

	"bitbucket.org/ossystems/agent/installmodes"
	"bitbucket.org/ossystems/agent/libarchive"
	"bitbucket.org/ossystems/agent/metadata"
	"bitbucket.org/ossystems/agent/utils"
)

type UbifsHelper interface {
	GetTargetDeviceFromUbiVolumeName(volume string) (string, error)
}

type UbifsHelperImpl struct {
	utils.CmdLineExecuter
	FileSystemBackend afero.Fs
}

func (uhi *UbifsHelperImpl) GetTargetDeviceFromUbiVolumeName(volume string) (string, error) {
	files, err := afero.ReadDir(uhi.FileSystemBackend, "/dev")
	if err != nil {
		return "", err
	}

	// foreach "/dev/ubi?" device node we check if the "volume"
	// is within this device node (we must run ubinfo on *device*
	// nodes, so "?" is to exclude *volume* nodes like "/dev/ubi0_1")
	prefix := "ubi"
	for _, file := range files {
		if !strings.HasPrefix(file.Name(), prefix) || len(file.Name()) != len(prefix)+1 {
			continue
		}

		deviceNumber := strings.Replace(file.Name(), "ubi", "", -1)

		// we can ignore the error here since we are dealing with
		// command execution over unknown ubi device nodes. we won't
		// get any collateral damage since we have a RE match right below
		combinedOutput, _ := uhi.Execute(fmt.Sprintf("ubinfo -d %s -N %s", deviceNumber, volume))

		// check if first line matches the RE below, if yes, then we found it
		scanner := bufio.NewScanner(strings.NewReader(string(combinedOutput)))
		scanner.Scan()

		r := regexp.MustCompile(`^Volume ID:   (\d) \(on ubi(\d)\)$`)
		matched := r.FindStringSubmatch(scanner.Text())

		if matched != nil && len(matched) == 3 {
			volumeID := matched[1]
			return fmt.Sprintf("/dev/ubi%s_%s", deviceNumber, volumeID), nil
		}
	}

	return "", fmt.Errorf("UBI volume '%s' wasn't found", volume)
}

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
	osfs := afero.NewOsFs()

	return &UbifsObject{
		CmdLineExecuter:   cle,
		Copier:            &utils.ExtendedIO{},
		LibArchiveBackend: &libarchive.LibArchive{},
		FileSystemBackend: osfs,
		UbifsHelper: &UbifsHelperImpl{
			CmdLineExecuter:   cle,
			FileSystemBackend: osfs,
		},
	}
}

type UbifsObject struct {
	metadata.ObjectMetadata
	metadata.CompressedObject
	utils.CmdLineExecuter
	UbifsHelper
	utils.Copier      `json:"-"`
	LibArchiveBackend libarchive.API `json:"-"`
	FileSystemBackend afero.Fs

	Target     string `json:"target"`
	TargetType string `json:"target-type"`
}

func (ufs *UbifsObject) Setup() error {
	if ufs.TargetType != "ubivolume" {
		return fmt.Errorf("target-type '%s' is not supported for the 'ubifs' handler. Its value must be 'ubivolume'", ufs.TargetType)
	}

	return nil
}

func (ufs *UbifsObject) Install() error {
	targetDevice, err := ufs.GetTargetDeviceFromUbiVolumeName(ufs.Target)
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

func (ufs *UbifsObject) Cleanup() error {
	return nil
}
