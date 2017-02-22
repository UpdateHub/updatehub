package imxkobs

import (
	"os/exec"
	"strconv"

	"bitbucket.org/ossystems/agent/installmodes"
	"bitbucket.org/ossystems/agent/metadata"
	"bitbucket.org/ossystems/agent/utils"
)

func init() {
	installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "imxkobs",
		CheckRequirements: checkRequirements,
		GetObject:         getObject,
	})
}

func checkRequirements() error {
	_, err := exec.LookPath("kobs-ng")

	return err
}

func getObject() interface{} {
	return &ImxKobsObject{
		CmdLineExecuter: &utils.CmdLine{},
	}
}

type ImxKobsObject struct {
	metadata.ObjectMetadata
	utils.CmdLineExecuter

	Add1KPadding    bool   `json:"1k_padding,omitempty"`
	SearchExponent  int    `json:"search_exponent,omitempty"`
	Chip0DevicePath string `json:"chip_0_device_path,omitempty"`
	Chip1DevicePath string `json:"chip_1_device_path,omitempty"`
}

func (ik *ImxKobsObject) Setup() error {
	return nil
}

func (ik *ImxKobsObject) Install() error {
	cmdline := "kobs-ng init"

	if ik.Add1KPadding {
		cmdline += " -x"
	}

	// FIXME: for cmdline we need to: path.Join(ik.UpdateDir, ik.Sha256sum)
	cmdline += " " + ik.Sha256sum

	if ik.SearchExponent > 0 {
		cmdline += " --search_exponent=" + strconv.Itoa(ik.SearchExponent)
	}

	if ik.Chip0DevicePath != "" {
		cmdline += " --chip_0_device_path=" + ik.Chip0DevicePath
	}

	if ik.Chip1DevicePath != "" {
		cmdline += " --chip_1_device_path=" + ik.Chip1DevicePath
	}

	cmdline += " -v"

	_, err := ik.Execute(cmdline)

	return err
}

func (ik *ImxKobsObject) Cleanup() error {
	return nil
}

// FIXME: install-different stuff?
