package imxkobs

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"bitbucket.org/ossystems/agent/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestImxKobsGetObject(t *testing.T) {
	ik, ok := getObject().(*ImxKobsObject)

	if !ok {
		t.Error("Failed to cast return value of \"getObject()\" to ImxKobsObject")
	}

	cmd := ik.CmdLine
	_, ok = cmd.(*utils.CmdLineImpl)

	if !ok {
		t.Error("Failed to cast default implementation of \"CmdLine\" to CmdLineImpl")
	}
}

func TestImxKobsCheckRequirementsWithKobsNGBinaryNotFound(t *testing.T) {
	// setup a temp dir on PATH
	test_path, err := ioutil.TempDir("", "imxkobs-test")
	assert.Nil(t, err)
	defer os.RemoveAll(test_path)
	os.Setenv("PATH", test_path)

	// test the call
	err = checkRequirements()

	assert.EqualError(t, err, "exec: \"kobs-ng\": executable file not found in $PATH")
}

func TestImxKobsCheckRequirementsWithKobsNGBinaryFound(t *testing.T) {
	// setup a temp dir on PATH
	test_path, err := ioutil.TempDir("", "imxkobs-test")
	assert.Nil(t, err)
	defer os.RemoveAll(test_path)
	os.Setenv("PATH", test_path)

	// setup the "kobs-ng" binary on PATH
	kobsng_path := path.Join(test_path, "kobs-ng")
	kobsng_file, err := os.Create(kobsng_path)
	assert.Nil(t, err)
	err = os.Chmod(kobsng_path, 0777)
	assert.Nil(t, err)
	defer kobsng_file.Close()

	// test the call
	err = checkRequirements()

	assert.NoError(t, err)
}

func TestImxKobsSetupNil(t *testing.T) {
	ik := ImxKobsObject{}
	assert.Nil(t, ik.Setup())
}

func TestImxKobsCleanupNil(t *testing.T) {
	ik := ImxKobsObject{}
	assert.Nil(t, ik.Cleanup())
}

type CmdLineMock struct {
	mock.Mock
}

func (clm CmdLineMock) Execute(cmdline string) ([]byte, error) {
	args := clm.Called(cmdline)
	return args.Get(0).([]byte), args.Error(1)
}

func TestImxKobsInstallSuccessCases(t *testing.T) {
	// FIXME: populate these fields with a json sample?
	testCases := []struct {
		Name            string
		Add1KPadding    bool
		SearchExponent  int
		Chip0DevicePath string
		Chip1DevicePath string
		ExpectedCmdLine string
	}{
		{
			"SuccessWithAllFields",
			true,
			1,
			"/dev/mtd0",
			"/dev/mtd1",
			"kobs-ng init -x a562ce06ed7398848eb910bb60c8c6f68ff36c33701afc30705a96d8eab12123 --search_exponent=1 --chip_0_device_path=/dev/mtd0 --chip_1_device_path=/dev/mtd1 -v",
		},
		{
			"SuccessWithoutAdd1kPadding",
			false,
			1,
			"/dev/mtd0",
			"/dev/mtd1",
			"kobs-ng init a562ce06ed7398848eb910bb60c8c6f68ff36c33701afc30705a96d8eab12123 --search_exponent=1 --chip_0_device_path=/dev/mtd0 --chip_1_device_path=/dev/mtd1 -v",
		},
		{
			"SuccessWithoutSearchExponent",
			true,
			0,
			"/dev/mtd0",
			"/dev/mtd1",
			"kobs-ng init -x a562ce06ed7398848eb910bb60c8c6f68ff36c33701afc30705a96d8eab12123 --chip_0_device_path=/dev/mtd0 --chip_1_device_path=/dev/mtd1 -v",
		},
		{
			"SuccessWithoutChip0DevicePath",
			true,
			1,
			"",
			"/dev/mtd1",
			"kobs-ng init -x a562ce06ed7398848eb910bb60c8c6f68ff36c33701afc30705a96d8eab12123 --search_exponent=1 --chip_1_device_path=/dev/mtd1 -v",
		},
		{
			"SuccessWithoutChip1DevicePath",
			true,
			1,
			"/dev/mtd0",
			"",
			"kobs-ng init -x a562ce06ed7398848eb910bb60c8c6f68ff36c33701afc30705a96d8eab12123 --search_exponent=1 --chip_0_device_path=/dev/mtd0 -v",
		},
		{
			"SuccessWithNegativeSearchExponent",
			true,
			-1,
			"/dev/mtd0",
			"/dev/mtd1",
			"kobs-ng init -x a562ce06ed7398848eb910bb60c8c6f68ff36c33701afc30705a96d8eab12123 --chip_0_device_path=/dev/mtd0 --chip_1_device_path=/dev/mtd1 -v",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			clm := CmdLineMock{}
			clm.On("Execute", tc.ExpectedCmdLine).Return([]byte("combined_output"), nil)

			ik := ImxKobsObject{CmdLine: clm}

			ik.Mode = "imxkobs"
			ik.Sha256sum = "a562ce06ed7398848eb910bb60c8c6f68ff36c33701afc30705a96d8eab12123"
			ik.Add1KPadding = tc.Add1KPadding
			ik.SearchExponent = tc.SearchExponent
			ik.Chip0DevicePath = tc.Chip0DevicePath
			ik.Chip1DevicePath = tc.Chip1DevicePath

			err := ik.Install()
			assert.NoError(t, err)

			clm.AssertExpectations(t)
		})
	}
}

func TestImxKobsInstallFailure(t *testing.T) {
	clm := CmdLineMock{}
	expected_cmdline := "kobs-ng init -x a562ce06ed7398848eb910bb60c8c6f68ff36c33701afc30705a96d8eab12123 --search_exponent=1 --chip_0_device_path=/dev/mtd0 --chip_1_device_path=/dev/mtd1 -v"
	clm.On("Execute", expected_cmdline).Return([]byte("combined_output"), fmt.Errorf("Error executing 'kobs-ng'. Output: combined_output"))

	ik := ImxKobsObject{CmdLine: clm}

	ik.Mode = "imxkobs"
	ik.Sha256sum = "a562ce06ed7398848eb910bb60c8c6f68ff36c33701afc30705a96d8eab12123"
	ik.Add1KPadding = true
	ik.SearchExponent = 1
	ik.Chip0DevicePath = "/dev/mtd0"
	ik.Chip1DevicePath = "/dev/mtd1"

	err := ik.Install()
	assert.EqualError(t, err, "Error executing 'kobs-ng'. Output: combined_output")

	clm.AssertExpectations(t)
}
