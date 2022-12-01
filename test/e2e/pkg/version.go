package e2e

// Set by the makefile
var pkgGitVersion string

func Version() string {
	if pkgGitVersion == "" {
		panic("version not set. Was this program not built using the e2e package's Makefile?")
	}
	return pkgGitVersion
}
