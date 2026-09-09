package cmd

import (
	"bufio"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withStdin(t *testing.T, input string) {
	t.Helper()
	original := stdinReader
	stdinReader = bufio.NewReader(strings.NewReader(input))
	t.Cleanup(func() { stdinReader = original })
}

// fmt.Scanln stopped at the first space, so `auth signup` recorded "Ada" for
// "Ada Lovelace" and left "Lovelace" to corrupt the next prompt.
func TestPromptLine_KeepsTheWholeLine(t *testing.T) {
	withStdin(t, "Ada Lovelace\nnext-value\n")

	name, err := promptLine("Name: ")
	require.NoError(t, err)
	assert.Equal(t, "Ada Lovelace", name)

	next, err := promptLine("Next: ")
	require.NoError(t, err)
	assert.Equal(t, "next-value", next)
}

// An empty line used to leave the field empty and let the command run on, so the
// server answered with a validation error about a flag the user never passed.
func TestPromptLine_RejectsEmptyInput(t *testing.T) {
	withStdin(t, "\n")

	_, err := promptLine("Email: ")

	require.ErrorIs(t, err, errNoInput)
	assert.Contains(t, err.Error(), "email")
}

func TestPromptLine_RejectsClosedStdin(t *testing.T) {
	withStdin(t, "")

	_, err := promptLine("Email: ")

	require.ErrorIs(t, err, errNoInput)
}

// term.ReadPassword fails with "inappropriate ioctl for device" when stdin is a
// pipe, which aborted every non-interactive login.
func TestPromptSecret_FallsBackWhenStdinIsNotATerminal(t *testing.T) {
	withStdin(t, "hunter2\n")

	secret, err := promptSecret("Password: ")

	require.NoError(t, err)
	assert.Equal(t, "hunter2", secret)
}

func TestFieldName(t *testing.T) {
	assert.Equal(t, "email", fieldName("Email: "))
	assert.Equal(t, "password", fieldName("Password: "))
}
