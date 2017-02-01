package copy

import (
	"fmt"
	"os"
	"path"
	"strings"

	"bitbucket.org/ossystems/agent/installmodes"
	"bitbucket.org/ossystems/agent/metadata"
	"bitbucket.org/ossystems/agent/utils"
)

func init() {
	installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "copy",
		CheckRequirements: func() error { return nil },
		GetObject: func() interface{} {
			return &CopyObject{FileSystemHelper: &utils.FileSystem{}, CustomCopier: &utils.CustomCopy{FileOperations: &utils.FileOperationsImpl{}}}
		},
	})
}

type CopyObject struct {
	metadata.ObjectMetadata
	metadata.CompressedObject
	utils.FileSystemHelper `json:"-"`
	utils.CustomCopier     `json:"-"`

	Target        string `json:"target"`
	TargetType    string `json:"target-type"`
	TargetPath    string `json:"target-path"`
	FSType        string `json:"filesystem"`
	FormatOptions string `json:"format-options,omitempty"`
	MustFormat    bool   `json:"format?,omitempty"`
	MountOptions  string `json:"mount-options,omitempty"`
	ChunkSize     int    `json:"chunk-size,omitempty"`
}

func (cp CopyObject) Setup() error {
	if cp.TargetType != "device" {
		return fmt.Errorf("target-type '%s' is not supported for the 'copy' handler. Its value must be 'device'", cp.TargetType)
	}

	return nil
}

func (cp CopyObject) Install() error {
	if cp.MustFormat {
		err := cp.Format(cp.Target, cp.FSType, cp.FormatOptions)
		if err != nil {
			return err
		}
	}

	tempDirPath, err := cp.TempDir("copy-handler")
	if err != nil {
		return err
	}
	// we can't "defer os.RemoveAll(tempDirPath)" here because it
	// could happen an "Umount" error and then the mounted dir
	// contents would be removed as well

	err = cp.Mount(cp.Target, tempDirPath, cp.FSType, cp.MountOptions)
	if err != nil {
		os.RemoveAll(tempDirPath)
		return err
	}

	targetPath := path.Join(tempDirPath, cp.TargetPath)
	cs := 128 * 1024
	if cp.ChunkSize > 0 {
		cs = cp.ChunkSize
	}

	errorList := []error{}

	// FIXME: on sourcePath we need to: path.Join(cp.UpdateDir, cp.Sha256sum)
	err = cp.CopyFile(cp.Sha256sum, targetPath, cs, 0, 0, -1, true, cp.Compressed)
	if err != nil {
		errorList = append(errorList, err)
	}

	umountErr := cp.Umount(tempDirPath)
	if umountErr != nil {
		errorList = append(errorList, umountErr)
	} else {
		os.RemoveAll(tempDirPath)
	}

	return mergeErrorList(errorList)
}

func (cp CopyObject) Cleanup() error {
	return nil
}

func mergeErrorList(errorList []error) error {
	if len(errorList) == 0 {
		return nil
	}

	if len(errorList) == 1 {
		return errorList[0]
	}

	errorMessages := []string{}
	for _, err := range errorList {
		errorMessages = append(errorMessages, fmt.Sprintf("(%v)", err))
	}

	return fmt.Errorf("%s", strings.Join(errorMessages[:], "; "))
}

// FIXME: install-different stuff
