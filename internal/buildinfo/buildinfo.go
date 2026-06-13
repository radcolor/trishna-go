package buildinfo

import "fmt"

var (
	Version = "dev"
	Commit  = "none"
	BuiltAt = "unknown"
)

func Label() string {
	if Commit == "none" {
		return Version
	}
	return fmt.Sprintf("%s (%s)", Version, Commit)
}
