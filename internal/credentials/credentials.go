// Package credentials decides which API key a command sends, and whether the
// environment running the CLI manages that key.
//
// A host that runs the CLI for many callers injects CAPIGO_API_KEY per caller.
// The variable being present — even empty — means the environment owns the
// key: the CLI then never stores one, never falls back to a stored one, and
// never tells the caller to log in. With CAPIGO_API_KEY and CAPIGO_NO_CONFIG
// both absent, Resolve keeps the precedence the CLI has always had.
package credentials

import (
	"os"

	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/config"
)

// EnvKey is the variable an environment injects the key through.
const EnvKey = "CAPIGO_API_KEY"

// Source names where the key came from. It is reported only in managed mode.
type Source string

const (
	SourceEnv    Source = "env"
	SourceConfig Source = "config"
	SourceNone   Source = "none"
)

// envKey reads CAPIGO_API_KEY directly. viper is not consulted: its
// AutomaticEnv with an empty prefix reads API_KEY first, which would let a
// caller override the key a host injected.
func envKey() (string, bool) {
	return os.LookupEnv(EnvKey)
}

// Managed reports whether the environment manages the key: CAPIGO_API_KEY is
// present with any value, or CAPIGO_NO_CONFIG is exactly "1".
func Managed() bool {
	_, present := envKey()
	return present || config.NoConfig()
}

// ManagedSource is the source a managed environment supplies the key from:
// env when CAPIGO_API_KEY is present, none otherwise. A managed key never
// comes from the config file.
func ManagedSource() Source {
	if _, present := envKey(); present {
		return SourceEnv
	}
	return SourceNone
}

// Resolve returns the key to send and where it came from, or the local error
// that explains why there is none. profileKey is the stored key; viperKey is
// the environment key as viper resolves it, used only outside managed mode.
func Resolve(profileKey, viperKey string) (string, Source, error) {
	if !Managed() {
		switch {
		case viperKey != "":
			return viperKey, SourceEnv, nil
		case profileKey != "":
			return profileKey, SourceConfig, nil
		}
		// Sending "Authorization: Bearer " with nothing after it costs a round
		// trip and comes back as AUTH_INVALID_FORMAT, which points at the wrong
		// repair. Diagnose it locally, as the condition it is.
		return "", SourceNone, &api.APIError{
			Code:           "AUTH_MISSING_HEADER",
			Message:        "no API key configured — this CLI has not been logged in (nothing in ~/.capigo/config.json, and CAPIGO_API_KEY is unset)",
			HTTPStatus:     401,
			LocalDiagnosis: true,
		}
	}

	key, present := envKey()
	if !present {
		return "", SourceNone, &api.APIError{
			Code:           "AUTH_MISSING_HEADER",
			Message:        "no API key supplied — CAPIGO_API_KEY is not set and the config file is disabled (CAPIGO_NO_CONFIG=1), so no request was sent",
			HTTPStatus:     401,
			LocalDiagnosis: true,
		}
	}
	if key == "" {
		// An empty variable is the environment saying "no key for this caller".
		// Falling back to a stored key would hand this caller someone else's.
		return "", SourceEnv, &api.APIError{
			Code:           "AUTH_EMPTY_ENV_KEY",
			Message:        "CAPIGO_API_KEY is set but empty — the environment supplied no key for this caller, so no request was sent and no stored key was used in its place",
			HTTPStatus:     401,
			LocalDiagnosis: true,
		}
	}
	return key, SourceEnv, nil
}

// KeyManagedByEnv refuses to store a key while the environment supplies one.
func KeyManagedByEnv() *api.APIError {
	return &api.APIError{
		Code:           "KEY_MANAGED_BY_ENV",
		Message:        "the API key is supplied by the environment (CAPIGO_API_KEY is set), so this CLI does not store one; nothing was written",
		LocalDiagnosis: true,
	}
}

// ConfigDisabled refuses to read or write the config file under CAPIGO_NO_CONFIG=1.
func ConfigDisabled() *api.APIError {
	return &api.APIError{
		Code:           "CONFIG_DISABLED",
		Message:        "the config file is disabled (CAPIGO_NO_CONFIG=1); nothing was read or written",
		LocalDiagnosis: true,
	}
}

// StoreKeyRefusal is the refusal for a command that would store a key, or nil
// when it may run. KEY_MANAGED_BY_ENV wins when both refusals apply.
func StoreKeyRefusal() *api.APIError {
	if _, present := envKey(); present {
		return KeyManagedByEnv()
	}
	return ConfigRefusal()
}

// ConfigRefusal is the refusal for a command that reads or writes the config
// file, or nil when it may run.
func ConfigRefusal() *api.APIError {
	if config.NoConfig() {
		return ConfigDisabled()
	}
	return nil
}
