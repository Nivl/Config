package userinput

import (
	"bufio"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFirstByte_Empty: empty line returns 0 (used by callers to detect
// the loop-and-retry case).
func TestFirstByte_Empty(t *testing.T) {
	assert.Equal(t, byte(0), firstByte(""))
}

// TestFirstByte_Y returns 'y'.
func TestFirstByte_Y(t *testing.T) {
	assert.Equal(t, byte('y'), firstByte("yes"))
}

// TestFirstByte_Capital returns 'Y' verbatim — callers do their own
// case-folding.
func TestFirstByte_Capital(t *testing.T) {
	assert.Equal(t, byte('Y'), firstByte("Yes"))
}

// TestReadLine_Newline strips trailing \n.
func TestReadLine_Newline(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("hello\n"))
	got, err := readLine(r)
	require.NoError(t, err)
	assert.Equal(t, "hello", got)
}

// TestReadLine_CRLF strips trailing \r\n.
func TestReadLine_CRLF(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("hello\r\n"))
	got, err := readLine(r)
	require.NoError(t, err)
	assert.Equal(t, "hello", got)
}

// TestReadLine_EOF: empty reader returns errNoInput.
func TestReadLine_EOF(t *testing.T) {
	r := bufio.NewReader(strings.NewReader(""))
	_, err := readLine(r)
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoInput)
}

// TestReadLine_EmptyLineNoEOF: a line containing just "\n" returns ""
// without erroring — caller-side loop semantics decide what to do.
func TestReadLine_EmptyLineNoEOF(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("\n"))
	got, err := readLine(r)
	require.NoError(t, err)
	assert.Empty(t, got)
}
