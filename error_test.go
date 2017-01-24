package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFatalError(t *testing.T) {
	err := NewFatalError(errors.New("fatal error"))

	assert.Error(t, err.Cause())
	assert.True(t, err.IsFatal())
}

func TestNewTransientError(t *testing.T) {
	err := NewTransientError(errors.New("transient error"))

	assert.Error(t, err.Cause())
	assert.False(t, err.IsFatal())
}
