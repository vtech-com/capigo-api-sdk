package config

import (
	"errors"
	"os"
)

// NoConfigEnv takes the config file out of the CLI entirely when it equals
// exactly "1". A host that injects a key per caller sets it so that no caller
// can read, plant or rewrite a key that the next caller would inherit.
const NoConfigEnv = "CAPIGO_NO_CONFIG"

// ErrDisabled is returned by Save when NoConfig is in force.
var ErrDisabled = errors.New("the config file is disabled (CAPIGO_NO_CONFIG=1)")

// NoConfig reports whether the config file is disabled. Only "1" counts:
// "true" or "0" read as unset, so a typo leaves today's behaviour in place
// rather than guessing at intent.
func NoConfig() bool {
	return os.Getenv(NoConfigEnv) == "1"
}
