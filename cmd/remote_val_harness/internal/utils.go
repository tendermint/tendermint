package internal

import (
	"os/user"
	"path/filepath"
	"strings"
)

// ExpandPath will check if the given path begins with a "~" symbol, and if so,
// will expand it to become the user's home directory.
func ExpandPath(path string) (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}

	if path == "~" {
		return usr.HomeDir, nil
	} else if strings.HasPrefix(path, "~/") {
		return filepath.Join(usr.HomeDir, path[2:]), nil
	}

	return path, nil
}
