package cmd

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveBool_FlagSetTrueOverridesEnv — explicit --personal=true
// returns true even when PERSONAL_COMPUTER had a value (no env read at
// all in this branch).
func TestResolveBool_FlagSetTrueOverridesEnv(t *testing.T) {
	t.Setenv("PERSONAL_COMPUTER", "")
	cmd, val := newBoolFlagCmd(t, "personal")
	parseArgs(t, cmd, "--personal=true")
	got, err := resolveBool(cmd, "personal", "PERSONAL_COMPUTER", *val, nil)
	require.NoError(t, err)
	assert.True(t, got)
}

// TestResolveBool_FlagSetFalseOverridesEnv — central correctness test
// for the .Changed semantic: --personal=false MUST beat env=true.
func TestResolveBool_FlagSetFalseOverridesEnv(t *testing.T) {
	t.Setenv("PERSONAL_COMPUTER", "true")
	cmd, val := newBoolFlagCmd(t, "personal")
	parseArgs(t, cmd, "--personal=false")
	got, err := resolveBool(cmd, "personal", "PERSONAL_COMPUTER", *val, nil)
	require.NoError(t, err)
	assert.False(t, got)
}

// TestResolveBool_FlagUnsetEnvWins — if the user didn't pass the flag,
// the env value is consulted (strict "true" comparison).
func TestResolveBool_FlagUnsetEnvWins(t *testing.T) {
	t.Setenv("PERSONAL_COMPUTER", "true")
	cmd, val := newBoolFlagCmd(t, "personal")
	parseArgs(t, cmd) // no args
	got, err := resolveBool(cmd, "personal", "PERSONAL_COMPUTER", *val, nil)
	require.NoError(t, err)
	assert.True(t, got)
}

// TestResolveBool_BothUnsetReturnsFalse — with nil fallback, the
// "neither flag nor env set" branch returns the zero value.
func TestResolveBool_BothUnsetReturnsFalse(t *testing.T) {
	t.Setenv("PERSONAL_COMPUTER", "")
	cmd, val := newBoolFlagCmd(t, "personal")
	parseArgs(t, cmd)
	got, err := resolveBool(cmd, "personal", "PERSONAL_COMPUTER", *val, nil)
	require.NoError(t, err)
	assert.False(t, got)
}

// TestResolveBool_FallbackInvokedWhenBothUnset — the fallback runs only
// in the "neither" branch and its result is returned.
func TestResolveBool_FallbackInvokedWhenBothUnset(t *testing.T) {
	t.Setenv("PERSONAL_COMPUTER", "")
	cmd, val := newBoolFlagCmd(t, "personal")
	parseArgs(t, cmd)
	got, err := resolveBool(cmd, "personal", "PERSONAL_COMPUTER", *val, func() (bool, error) {
		return true, nil
	})
	require.NoError(t, err)
	assert.True(t, got)
}

// TestResolveBool_FallbackSkippedWhenEnvSet — env wins over fallback;
// fallback is never invoked. Useful invariant for expensive fallbacks
// (e.g., a shellout) we don't want to run unnecessarily.
func TestResolveBool_FallbackSkippedWhenEnvSet(t *testing.T) {
	t.Setenv("PERSONAL_COMPUTER", "true")
	cmd, val := newBoolFlagCmd(t, "personal")
	parseArgs(t, cmd)
	called := false
	got, err := resolveBool(cmd, "personal", "PERSONAL_COMPUTER", *val, func() (bool, error) {
		called = true
		return false, nil
	})
	require.NoError(t, err)
	assert.True(t, got)
	assert.False(t, called, "fallback must not run when env already set")
}

// TestResolveString_FlagSetOverridesEnv — explicit flag wins.
func TestResolveString_FlagSetOverridesEnv(t *testing.T) {
	t.Setenv("DEV_ROOT", "/from/env")
	cmd, val := newStringFlagCmd(t, "dev-root")
	parseArgs(t, cmd, "--dev-root=/from/flag")
	got, err := resolveString(cmd, "dev-root", "DEV_ROOT", *val, nil)
	require.NoError(t, err)
	assert.Equal(t, "/from/flag", got)
}

// TestResolveString_FlagUnsetEnvWins — env survives when the flag is
// not passed.
func TestResolveString_FlagUnsetEnvWins(t *testing.T) {
	t.Setenv("GIT_HOST", "git@github.com")
	cmd, val := newStringFlagCmd(t, "git-host")
	parseArgs(t, cmd)
	got, err := resolveString(cmd, "git-host", "GIT_HOST", *val, nil)
	require.NoError(t, err)
	assert.Equal(t, "git@github.com", got)
}

// TestResolveString_ExplicitEmptyDoesNotClobber — `--git-host=""`
// should not wipe a useful env value. Cobra string defaults are empty;
// promoting "" would mean default-passing accidentally clobbers env.
func TestResolveString_ExplicitEmptyDoesNotClobber(t *testing.T) {
	t.Setenv("GIT_HOST", "git@github.com")
	cmd, val := newStringFlagCmd(t, "git-host")
	parseArgs(t, cmd, "--git-host=")
	got, err := resolveString(cmd, "git-host", "GIT_HOST", *val, nil)
	require.NoError(t, err)
	assert.Equal(t, "git@github.com", got)
}

// TestResolveString_FallbackPropagatesError — when both flag and env
// are unset and the fallback errors (e.g., `brew --prefix` failure),
// the error surfaces verbatim so the caller can wrap it with a
// useful message.
func TestResolveString_FallbackPropagatesError(t *testing.T) {
	t.Setenv("HOMEBREW_PREFIX", "")
	cmd, val := newStringFlagCmd(t, "homebrew-prefix")
	parseArgs(t, cmd)
	wantErr := errors.New("brew missing")
	_, err := resolveString(cmd, "homebrew-prefix", "HOMEBREW_PREFIX", *val, func() (string, error) {
		return "", wantErr
	})
	require.ErrorIs(t, err, wantErr)
}

// TestResolveString_FallbackSkippedWhenFlagSet — covers the lazy-
// evaluation contract: an expensive fallback (e.g., a shellout) must
// not run when the user explicitly provided the flag.
func TestResolveString_FallbackSkippedWhenFlagSet(t *testing.T) {
	t.Setenv("HOMEBREW_PREFIX", "")
	cmd, val := newStringFlagCmd(t, "homebrew-prefix")
	parseArgs(t, cmd, "--homebrew-prefix=/opt/homebrew")
	called := false
	got, err := resolveString(cmd, "homebrew-prefix", "HOMEBREW_PREFIX", *val, func() (string, error) {
		called = true
		return "", errors.New("should not run")
	})
	require.NoError(t, err)
	assert.Equal(t, "/opt/homebrew", got)
	assert.False(t, called, "fallback must not run when flag is set")
}

// TestResolveBoolAsString_FlagSetReturnsCanonicalString — covers the
// userinput-bridge helper that turns a bool flag's .Changed state into
// a tri-state string for the prompt-helper prearg.
func TestResolveBoolAsString_FlagSetReturnsCanonicalString(t *testing.T) {
	t.Setenv("PERSONAL_COMPUTER", "")
	cmd, val := newBoolFlagCmd(t, "personal")
	parseArgs(t, cmd, "--personal=false")
	assert.Equal(t, "false", resolveBoolAsString(cmd, "personal", "PERSONAL_COMPUTER", *val))
}

// TestResolveBoolAsString_FlagUnsetReturnsEnv — env value passes
// through verbatim (including garbage values; there is no validation
// on pre-set); empty env returns empty so downstream userinput.X
// prompts.
func TestResolveBoolAsString_FlagUnsetReturnsEnv(t *testing.T) {
	t.Setenv("PERSONAL_COMPUTER", "garbage")
	cmd, val := newBoolFlagCmd(t, "personal")
	parseArgs(t, cmd)
	assert.Equal(t, "garbage", resolveBoolAsString(cmd, "personal", "PERSONAL_COMPUTER", *val))
}

// TestResolveBoolAsString_BothUnsetReturnsEmpty — empty signals
// "prompt the user" downstream.
func TestResolveBoolAsString_BothUnsetReturnsEmpty(t *testing.T) {
	t.Setenv("PERSONAL_COMPUTER", "")
	cmd, val := newBoolFlagCmd(t, "personal")
	parseArgs(t, cmd)
	assert.Empty(t, resolveBoolAsString(cmd, "personal", "PERSONAL_COMPUTER", *val))
}

// TestResolveBoolAsString_FlagSetTrueReturnsTrue — explicit
// --personal=true covers the bool-true branch of resolveBoolAsString;
// the sibling _FlagSetReturnsCanonicalString test covers the
// bool-false branch.
func TestResolveBoolAsString_FlagSetTrueReturnsTrue(t *testing.T) {
	t.Setenv("PERSONAL_COMPUTER", "")
	cmd, val := newBoolFlagCmd(t, "personal")
	parseArgs(t, cmd, "--personal=true")
	assert.Equal(t, "true", resolveBoolAsString(cmd, "personal", "PERSONAL_COMPUTER", *val))
}

// newBoolFlagCmd is a test helper that builds a throwaway cobra command
// carrying a single bool flag, returning the command and a pointer to
// the parsed value.
func newBoolFlagCmd(t *testing.T, name string) (cmd *cobra.Command, value *bool) { //nolint:unparam // mirrors newStringFlagCmd; only one bool flag in use today ("personal") but kept parameterized for symmetry + future bool knobs.
	t.Helper()
	cmd = &cobra.Command{Use: "test"}
	value = cmd.Flags().Bool(name, false, "")
	return cmd, value
}

// newStringFlagCmd is the string-flag analogue of newBoolFlagCmd.
func newStringFlagCmd(t *testing.T, name string) (cmd *cobra.Command, value *string) {
	t.Helper()
	cmd = &cobra.Command{Use: "test"}
	value = cmd.Flags().String(name, "", "")
	return cmd, value
}

// parseArgs runs the cobra flag parser on the given args slice, which
// is what cobra normally does internally before invoking RunE. Tests
// need it to flip the .Changed bit on flags they want to simulate as
// "user-supplied."
func parseArgs(t *testing.T, c *cobra.Command, args ...string) {
	t.Helper()
	c.SetArgs(args)
	if err := c.Flags().Parse(args); err != nil {
		t.Fatalf("parse flags %v: %v", args, err)
	}
}
