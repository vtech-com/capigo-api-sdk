package cmd

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runCapigoStdin is runCapigo with a piped stdin, which is how a script or an agent hands the
// login key over without it landing in a shell history.
func runCapigoStdin(t *testing.T, env []string, stdin string, args ...string) cliResult {
	t.Helper()
	cmd := exec.Command(envkeyBinary(t), args...)
	cmd.Env = env
	cmd.Dir = t.TempDir()
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run capigo %v: %v", args, err)
		}
		code = exitErr.ExitCode()
	}
	return cliResult{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func TestLoginKeyStdinStoresTheKey(t *testing.T) {
	f := newFakeAPI(t)
	home := t.TempDir()
	r := runCapigoStdin(t, baseEnv(home, f), "csk_from_stdin\n", "auth", "login", "--key-stdin")
	if r.code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout: %s\nstderr: %s", r.code, r.stdout, r.stderr)
	}
	if strings.Contains(r.stdout+r.stderr, "csk_from_stdin") {
		t.Errorf("the key was echoed back:\n%s\n%s", r.stdout, r.stderr)
	}
	p := readProfile(t, filepath.Join(home, ".capigo", "config.json"))
	if err := wantField(p, "api_key", "csk_from_stdin"); err != nil {
		t.Error(err)
	}
}

func TestLoginKeyStdinTrimsSurroundingWhitespace(t *testing.T) {
	f := newFakeAPI(t)
	home := t.TempDir()
	r := runCapigoStdin(t, baseEnv(home, f), "  csk_padded \r\n", "auth", "login", "--key-stdin")
	if r.code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", r.code, r.stdout)
	}
	p := readProfile(t, filepath.Join(home, ".capigo", "config.json"))
	if err := wantField(p, "api_key", "csk_padded"); err != nil {
		t.Error(err)
	}
}

func TestLoginRefusesInputsThatAreNotAKey(t *testing.T) {
	cases := map[string]struct {
		stdin string
		args  []string
	}{
		"no key and stdin is not a terminal": {"", []string{"auth", "login"}},
		"empty stdin":                        {"", []string{"auth", "login", "--key-stdin"}},
		"stdin key without the csk_ prefix":  {"not-a-key\n", []string{"auth", "login", "--key-stdin"}},
		"both --key and --key-stdin":         {"csk_a\n", []string{"auth", "login", "--key", "csk_b", "--key-stdin"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFakeAPI(t)
			home := t.TempDir()
			r := runCapigoStdin(t, baseEnv(home, f), c.stdin, c.args...)
			if r.code != 1 {
				t.Errorf("exit = %d, want 1\nstdout: %s", r.code, r.stdout)
			}
			if got := r.errorField(t, "code"); got != "INVALID_API_KEY" {
				t.Errorf("code = %q, want INVALID_API_KEY", got)
			}
			assertNoConfigDir(t, home)
			assertNoRequests(t, f)
		})
	}
}

func TestLoginKeyStdinRefusedWhenEnvKeyPresent(t *testing.T) {
	f := newFakeAPI(t)
	home := t.TempDir()
	r := runCapigoStdin(t, baseEnv(home, f, "CAPIGO_API_KEY=csk_x"), "csk_y\n", "auth", "login", "--key-stdin")
	assertRefused(t, r, 1, "KEY_MANAGED_BY_ENV", "env")
	assertNoConfigDir(t, home)
}

// The hidden prompt cannot be driven through a pipe, so its two outcomes are checked in process.
func TestResolveLoginKeyPromptsWhenStdinIsATerminal(t *testing.T) {
	prevTTY, prevRead := stdinIsTerminal, readHiddenLine
	t.Cleanup(func() { stdinIsTerminal, readHiddenLine = prevTTY, prevRead })
	stdinIsTerminal = func() bool { return true }
	readHiddenLine = func() (string, error) { return " csk_typed \n", nil }

	var prompt bytes.Buffer
	got := resolveLoginKey("", false, strings.NewReader(""), &prompt)
	if got != "csk_typed" {
		t.Errorf("key = %q, want csk_typed", got)
	}
	if !strings.Contains(prompt.String(), "hidden") {
		t.Errorf("prompt did not say the input is hidden: %q", prompt.String())
	}
	if strings.Contains(prompt.String(), "csk_typed") {
		t.Errorf("the prompt echoed the key: %q", prompt.String())
	}
}

func TestResolveLoginKeyKeepsTheFlagValueWithoutPrompting(t *testing.T) {
	prevTTY := stdinIsTerminal
	t.Cleanup(func() { stdinIsTerminal = prevTTY })
	stdinIsTerminal = func() bool { t.Error("prompted although --key was given"); return true }

	var prompt bytes.Buffer
	if got := resolveLoginKey("csk_flag", false, strings.NewReader(""), &prompt); got != "csk_flag" {
		t.Errorf("key = %q, want csk_flag", got)
	}
	if prompt.Len() != 0 {
		t.Errorf("wrote to the prompt stream: %q", prompt.String())
	}
}
