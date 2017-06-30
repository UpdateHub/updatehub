/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package activeinactive

import (
	"fmt"
	"testing"

	"github.com/UpdateHub/updatehub/testsmocks/cmdlinemock"
	"github.com/stretchr/testify/assert"
)

func TestDefaultImplActive(t *testing.T) {
	clm := &cmdlinemock.CmdLineExecuterMock{}
	clm.On("Execute", "updatehub-active-get").Return([]byte("1"), nil)

	di := DefaultImpl{
		CmdLineExecuter: clm,
	}

	active, err := di.Active()

	assert.NoError(t, err)
	assert.Equal(t, 1, active)

	clm.AssertExpectations(t)
}

func TestDefaultImplActiveWithExecuteError(t *testing.T) {
	clm := &cmdlinemock.CmdLineExecuterMock{}
	clm.On("Execute", "updatehub-active-get").Return([]byte(""), fmt.Errorf("execute error"))

	di := DefaultImpl{
		CmdLineExecuter: clm,
	}

	active, err := di.Active()

	assert.EqualError(t, err, "failed to execute 'updatehub-active-get': execute error")
	assert.Equal(t, 0, active)

	clm.AssertExpectations(t)
}

func TestDefaultImplActiveWithParseIntError(t *testing.T) {
	clm := &cmdlinemock.CmdLineExecuterMock{}
	clm.On("Execute", "updatehub-active-get").Return([]byte("a"), nil)

	di := DefaultImpl{
		CmdLineExecuter: clm,
	}

	active, err := di.Active()

	assert.EqualError(t, err, "failed to parse response from 'updatehub-active-get': strconv.ParseInt: parsing \"a\": invalid syntax")
	assert.Equal(t, 0, active)

	clm.AssertExpectations(t)
}

func TestDefaultImplSetActive(t *testing.T) {
	clm := &cmdlinemock.CmdLineExecuterMock{}
	clm.On("Execute", "updatehub-active-set 1").Return([]byte(""), nil)

	di := DefaultImpl{
		CmdLineExecuter: clm,
	}

	err := di.SetActive(1)

	assert.NoError(t, err)
	clm.AssertExpectations(t)
}

func TestDefaultImplSetActiveWithExecuteError(t *testing.T) {
	clm := &cmdlinemock.CmdLineExecuterMock{}
	clm.On("Execute", "updatehub-active-set 1").Return([]byte(""), fmt.Errorf("execute error"))

	di := DefaultImpl{
		CmdLineExecuter: clm,
	}

	err := di.SetActive(1)

	assert.EqualError(t, err, "failed to execute 'updatehub-active-set': execute error")
	clm.AssertExpectations(t)
}
