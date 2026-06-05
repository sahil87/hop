package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNoConfig is returned when the fixed config path does not resolve. Retained
// (exported) for compatibility; the actual returned errors use fmt.Errorf with
// the messages below (callers don't currently errors.Is the sentinel).
var ErrNoConfig = errors.New("hop: no hop.yaml found")

// configPath returns the single, fixed config location. The only environment
// input is $HOME (unavoidable). No $HOP_CONFIG, no $XDG_CONFIG_HOME — the path
// is identical on macOS and Linux by construction (we build it with
// filepath.Join rather than os.UserConfigDir, which would resolve to
// ~/Library/Application Support on macOS).
func configPath() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return "", fmt.Errorf("hop: $HOME is not set; cannot locate config")
	}
	return filepath.Join(home, ".config", "hop", "hop.yaml"), nil
}

// Resolve returns the fixed config path $HOME/.config/hop/hop.yaml when the file
// exists, or a not-found error otherwise. Used by every read path.
func Resolve() (string, error) {
	p, err := configPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("hop: no hop.yaml found at %s. Run 'hop add <dir>' to register a repo (creates the config), or 'hop config init' for a starter.", p)
		}
		return "", fmt.Errorf("hop: stat %s: %w", p, err)
	}
	return p, nil
}

// ResolveWriteTarget returns the path that would be used as the config target
// for hop config init / hop config where. Unlike Resolve, this does NOT stat the
// file — it returns the path that *would* be used regardless of whether the file
// currently exists. Kept as a distinct function (wrapping configPath) so the
// no-stat seam init/where rely on is preserved.
//
// Returns an error only when the path cannot be determined (no $HOME).
func ResolveWriteTarget() (string, error) {
	return configPath()
}
