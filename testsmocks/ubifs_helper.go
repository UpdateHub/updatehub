package testsmocks

import "github.com/stretchr/testify/mock"

type UbifsHelperMock struct {
	mock.Mock
}

func (uhm *UbifsHelperMock) GetTargetDeviceFromUbiVolumeName(volume string) (string, error) {
	args := uhm.Called(volume)
	return args.String(0), args.Error(1)
}
