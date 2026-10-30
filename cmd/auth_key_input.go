package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Seams for tests: a real terminal cannot be driven from a unit test.
var (
	stdinIsTerminal = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }
	readHiddenLine  = func() (string, error) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		return string(b), err
	}
)

// resolveLoginKey returns the key `auth login` should store, from the first source that applies:
// --key, --key-stdin, or a hidden prompt on a terminal. The key never reaches a shell history or
// a process list unless the caller chose --key. It exits through fail on any input problem, so a
// caller never has to render the error itself.
func resolveLoginKey(flagKey string, fromStdin bool, stdin io.Reader, prompt io.Writer) string {
	switch {
	case flagKey != "" && fromStdin:
		fail("INVALID_API_KEY", "use either --key or --key-stdin, not both", 0)
	case flagKey != "":
		return flagKey
	case fromStdin:
		line, err := bufio.NewReader(stdin).ReadString('\n')
		if err != nil && err != io.EOF {
			fail("INVALID_API_KEY", "could not read the API key from stdin", 0)
		}
		return strings.TrimSpace(line)
	case stdinIsTerminal():
		fmt.Fprint(prompt, "Paste your Capigo API key (input is hidden): ")
		line, err := readHiddenLine()
		fmt.Fprintln(prompt)
		if err != nil {
			fail("INVALID_API_KEY", "could not read the API key from the terminal", 0)
		}
		return strings.TrimSpace(line)
	}
	fail("INVALID_API_KEY", "no API key given: run this in a terminal to be prompted, or pipe the key in with --key-stdin", 0)
	return ""
}
