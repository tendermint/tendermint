package version

const Maj = "0"
const Min = "10"
const Fix = "5"

var (
	// The full version string
	Version = "0.10.5"

	// GitCommit is set with --ldflags "-X main.gitCommit=$(git rev-parse HEAD)"
	GitCommit string
)

func init() {
	if GitCommit != "" {
		Version += "-" + GitCommit[:8]
	}
}
