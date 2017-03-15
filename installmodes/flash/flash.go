package flash

import (
	"fmt"
	"os/exec"

	"github.com/spf13/afero"

	"code.ossystems.com.br/updatehub/agent/installmodes"
	"code.ossystems.com.br/updatehub/agent/metadata"
	"code.ossystems.com.br/updatehub/agent/utils"
)

func init() {
	installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "flash",
		CheckRequirements: checkRequirements,
		GetObject:         getObject,
	})
}

func checkRequirements() error {
	for _, binary := range []string{"nandwrite", "flashcp", "flash_erase"} {
		_, err := exec.LookPath(binary)
		if err != nil {
			return err
		}
	}

	return nil
}

func getObject() interface{} {
	return &FlashObject{
		CmdLineExecuter:   &utils.CmdLine{},
		FileSystemBackend: afero.NewOsFs(),
		MtdUtils:          &utils.MtdUtilsImpl{},
	}
}

// FlashObject encapsulates the "flash" handler data and functions
type FlashObject struct {
	metadata.ObjectMetadata
	utils.CmdLineExecuter
	FileSystemBackend afero.Fs
	utils.MtdUtils

	Target     string `json:"target"`
	TargetType string `json:"target-type"`

	targetDevice string // this is NOT obtained from the json but from the "Setup()"
}

// Setup implementation for the "flash" handler
func (f *FlashObject) Setup() error {
	switch f.TargetType {
	case "device":
		f.targetDevice = f.Target
	case "mtdname":
		td, err := f.MtdUtils.GetTargetDeviceFromMtdName(f.FileSystemBackend, f.Target)
		if err != nil {
			return err
		}

		f.targetDevice = td
	default:
		return fmt.Errorf("target-type '%s' is not supported for the 'flash' handler. Its value must be either 'device' or 'mtdname'", f.TargetType)
	}

	return nil
}

// Install implementation for the "flash" handler
func (f *FlashObject) Install() error {
	isNand, err := f.MtdUtils.MtdIsNAND(f.targetDevice)
	if err != nil {
		return err
	}

	_, err = f.Execute(fmt.Sprintf("flash_erase %s 0 0", f.targetDevice))
	if err != nil {
		return err
	}

	// FIXME: for srcPath we need to: path.Join(f.UpdateDir, f.Sha256sum)
	srcPath := f.Sha256sum

	if isNand {
		_, nandErr := f.Execute(fmt.Sprintf("nandwrite -p %s %s", f.targetDevice, srcPath))
		err = nandErr
	} else {
		_, norErr := f.Execute(fmt.Sprintf("flashcp %s %s", srcPath, f.targetDevice))
		err = norErr
	}

	return err
}

// Cleanup implementation for the "flash" handler
func (f *FlashObject) Cleanup() error {
	return nil
}

// FIXME: install-different stuff?
