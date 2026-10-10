package hooks

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// UserConfiguration locates each host's user configuration directory in one
// environment: the hooks a host finds there apply to every project it opens.
type UserConfiguration struct {
	// Windows marks the Windows profile, whose files the Windows host apps
	// read when they open a project inside the WSL distribution.
	Windows bool
	// Directories maps a host name to its user configuration directory.
	Directories map[string]string
}

// NativeUserConfiguration is this environment's user configuration: each
// host's directory under home, unless the host's own variable moves it.
func NativeUserConfiguration(home string, getenv func(string) string) UserConfiguration {
	directories := map[string]string{}
	for _, host := range hookHosts {
		directory := getenv(host.homeVariable)
		if directory == "" {
			directory = filepath.Join(home, host.homeDirectory)
		}
		directories[host.name] = directory
	}
	return UserConfiguration{Directories: directories}
}

// UserConfigurations returns the user configurations an install writes:
// this environment's, and inside a WSL distribution also the Windows
// profile's. They are read when an install or status needs them, since
// reaching Windows starts cmd.exe.
func UserConfigurations(distro string) func() ([]UserConfiguration, error) {
	return func() ([]UserConfiguration, error) {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("user configuration: %w", err)
		}
		configurations := make([]UserConfiguration, 0, 2)
		configurations = append(configurations, NativeUserConfiguration(home, os.Getenv))
		if distro == "" {
			return configurations, nil
		}
		windows, err := windowsUserConfiguration()
		if err != nil {
			return nil, fmt.Errorf("windows user configuration: %w", err)
		}
		return append(configurations, windows), nil
	}
}

// windowsTimeout bounds each call into Windows, which starts a process
// outside the distribution.
const windowsTimeout = 30 * time.Second

// windowsUserConfiguration reads the Windows environment through cmd.exe
// and maps each host's directory into the distribution with wslpath.
func windowsUserConfiguration() (UserConfiguration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), windowsTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, "cmd.exe", "/d", "/c", "set").Output()
	if err != nil {
		return UserConfiguration{}, fmt.Errorf("read the Windows environment through cmd.exe: %w", err)
	}
	variables := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		name, value, found := strings.Cut(strings.TrimRight(scanner.Text(), "\r"), "=")
		if found {
			variables[strings.ToUpper(name)] = value
		}
	}
	profile := variables["USERPROFILE"]
	if profile == "" {
		return UserConfiguration{}, errors.New("the Windows environment has no USERPROFILE")
	}
	directories := map[string]string{}
	for _, host := range hookHosts {
		directory := variables[strings.ToUpper(host.homeVariable)]
		if directory == "" {
			directory = profile + `\` + host.homeDirectory
		}
		mapped, err := exec.CommandContext(ctx, "wslpath", "-u", directory).Output()
		if err != nil {
			return UserConfiguration{}, fmt.Errorf("map %s into the distribution: %w", directory, err)
		}
		directories[host.name] = strings.TrimSpace(string(mapped))
	}
	return UserConfiguration{Windows: true, Directories: directories}, nil
}

// InvokedBinary is the absolute path of the arclint that was run as arg0:
// the path itself when it names one, otherwise where PATH found it. Links
// stay unresolved, so a hook keeps following an upgrade behind a link.
func InvokedBinary(arg0 string) (string, error) {
	path := arg0
	if !strings.ContainsRune(arg0, '/') && !strings.ContainsRune(arg0, filepath.Separator) {
		found, err := exec.LookPath(arg0)
		if err != nil {
			return "", fmt.Errorf("locate the running arclint: %w", err)
		}
		path = found
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("locate the running arclint: %w", err)
	}
	return absolute, nil
}
