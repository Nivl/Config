package appsetuptest

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestFakeCmdRunner_RunReturnsExpectedError verifies the fake routes
// through testify's mock and returns the configured error.
func TestFakeCmdRunner_RunReturnsExpectedError(t *testing.T) {
	fake := NewFakeCmdRunner()
	want := errors.New("simulated failure")
	fake.On("Run", mock.Anything, "ssh-keygen", "-foo").Return(want)

	err := fake.Run(context.Background(), "ssh-keygen", "-foo")
	require.Error(t, err)
	assert.Equal(t, want, err)
	fake.AssertExpectations(t)
}

// TestFakeCmdRunner_RunNilError verifies success path.
func TestFakeCmdRunner_RunNilError(t *testing.T) {
	fake := NewFakeCmdRunner()
	fake.On("Run", mock.Anything, "gh", "auth", "login", "-w").Return(nil)

	err := fake.Run(context.Background(), "gh", "auth", "login", "-w")
	require.NoError(t, err)
	fake.AssertExpectations(t)
}

// TestFakeCmdRunner_CaptureReturnsBytes verifies Capture round-trips
// stdout bytes through the mock.
func TestFakeCmdRunner_CaptureReturnsBytes(t *testing.T) {
	fake := NewFakeCmdRunner()
	want := []byte(`{"hosts":{"github.com":[{"state":"success"}]}}`)
	fake.On("Capture", mock.Anything, "gh", "auth", "status", "-a", "--json", "hosts").Return(want, nil)

	got, err := fake.Capture(context.Background(), "gh", "auth", "status", "-a", "--json", "hosts")
	require.NoError(t, err)
	assert.Equal(t, want, got)
	fake.AssertExpectations(t)
}

// TestFakeCmdRunner_CaptureReturnsError verifies error propagation
// from Capture.
func TestFakeCmdRunner_CaptureReturnsError(t *testing.T) {
	fake := NewFakeCmdRunner()
	want := errors.New("gh not found")
	fake.On("Capture", mock.Anything, "gh", "auth", "status", "-a", "--json", "hosts").Return([]byte(nil), want)

	got, err := fake.Capture(context.Background(), "gh", "auth", "status", "-a", "--json", "hosts")
	require.Error(t, err)
	assert.Equal(t, want, err)
	assert.Empty(t, got)
}
