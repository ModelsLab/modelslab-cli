package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// errNoInput is returned when a prompt gets nothing to work with.
var errNoInput = errors.New("no input provided")

// stdinReader is shared so a secret read from a pipe does not lose whatever the
// next prompt was going to read. bufio would swallow it in its own buffer.
var stdinReader = bufio.NewReader(os.Stdin)

// promptLine reads one whole line.
//
// It replaces fmt.Scanln, which had two failure modes here: it stops at the
// first space, so `auth signup` recorded "Ada" for "Ada Lovelace" and left the
// rest to corrupt the next prompt; and on an empty line it returns an error the
// caller ignored, so the command carried on with an empty email and the server
// answered with a validation error about a flag the user never passed.
func promptLine(label string) (string, error) {
	fmt.Fprint(os.Stderr, label)

	line, err := stdinReader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("could not read %s: %w", fieldName(label), err)
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return "", fmt.Errorf("%s: %w", fieldName(label), errNoInput)
	}

	return line, nil
}

// promptSecret reads a secret without echoing it.
//
// When stdin is not a terminal — a pipe, a heredoc, CI — term.ReadPassword fails
// with "inappropriate ioctl for device", which used to abort the login entirely.
// The only remaining non-interactive option was --password on the command line,
// which lands in shell history, ps output and CI logs. Fall back to reading the
// line instead; there is no echo to suppress when nobody is typing.
func promptSecret(label string) (string, error) {
	if !term.IsTerminal(int(syscall.Stdin)) {
		return promptLine(label)
	}

	fmt.Fprint(os.Stderr, label)
	raw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("could not read %s: %w", fieldName(label), err)
	}

	secret := strings.TrimRight(string(raw), "\r\n")
	if secret == "" {
		return "", fmt.Errorf("%s: %w", fieldName(label), errNoInput)
	}

	return secret, nil
}

// fieldName turns a prompt label ("Email: ") into something an error can read
// naturally ("email").
func fieldName(label string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(label), ":")))
}
