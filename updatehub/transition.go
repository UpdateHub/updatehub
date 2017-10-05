/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"bytes"
	"strconv"
	"strings"
)

// TransitionFlow indicates the transition flow of state change callback
type TransitionFlow int

const (
	// TransitionFlowUnchanged indicates that transition flow will remain unchanged
	TransitionFlowUnchanged = iota
	// TransitionFlowCancelled indicates that transition flow will be cancelled
	TransitionFlowCancelled
	// TransitionFlowPostponed indicates that transition flow will be postponed
	TransitionFlowPostponed
)

// DetermineTransitionFlow determines transition flow for state change callback
func DetermineTransitionFlow(output []byte) (TransitionFlow, interface{}) {
	parts := strings.Split(string(bytes.TrimSpace(output)), " ")

	switch parts[0] {
	case "cancel":
		return TransitionFlowCancelled, nil
	case "try_again":
		seconds, err := strconv.Atoi(parts[1])
		if err != nil {
			return TransitionFlowUnchanged, nil
		}

		return TransitionFlowPostponed, seconds
	}

	return TransitionFlowUnchanged, nil
}
