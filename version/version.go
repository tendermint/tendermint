package version

const Maj = "0"
const Min = "11"
const Fix = "1"

var (
	// The full version string
	Version = "0.11.1"

	// GitCommit is set with --ldflags "-X main.gitCommit=$(git rev-parse HEAD)"
	GitCommit string
)

func init() {
	if GitCommit != "" {
		Version += "-" + GitCommit[:8]
	}
}
