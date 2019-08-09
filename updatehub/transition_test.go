/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package updatehub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransitionFlowUnchanged(t *testing.T) {
	flow, args := DetermineTransitionFlow([]byte(""))

	assert.Equal(t, TransitionFlow(TransitionFlowUnchanged), flow)
	assert.Nil(t, args)
}

func TestTransitionFlowCancelled(t *testing.T) {
	flow, args := DetermineTransitionFlow([]byte("cancel"))

	assert.Equal(t, TransitionFlow(TransitionFlowCancelled), flow)
	assert.Nil(t, args)
}

func TestTransitionFlowPostponed(t *testing.T) {
	flow, args := DetermineTransitionFlow([]byte("try_again 13"))

	assert.Equal(t, TransitionFlow(TransitionFlowPostponed), flow)
	assert.Equal(t, args.(int), 13)
}

func TestTransitionFlowPostponedWithInvalidArgument(t *testing.T) {
	flow, args := DetermineTransitionFlow([]byte("try_again x"))

	assert.Equal(t, TransitionFlow(TransitionFlowUnchanged), flow)
	assert.Nil(t, args)
}
