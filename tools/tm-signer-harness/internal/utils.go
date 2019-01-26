package internal

import (
	"os/user"
	"path/filepath"
	"strings"
)

// ExpandPath will check if the given path begins with a "~" symbol, and if so,
// will expand it to become the user's home directory. If it fails to expand the
// path it will automatically return the original path itself.
func ExpandPath(path string) string {
	usr, err := user.Current()
	if err != nil {
		return path
	}

	if path == "~" {
		return usr.HomeDir
	} else if strings.HasPrefix(path, "~/") {
		return filepath.Join(usr.HomeDir, path[2:])
	}

	return path
}
