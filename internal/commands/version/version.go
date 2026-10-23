package version

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// commitFromPseudoVersion extracts the 12-char commit hash from the end of a
// Go pseudo-version, e.g. v0.0.0-20261023144319-c140a493734d -> c140a493734d.
// Returns "" if version isn't in that form.
func commitFromPseudoVersion(version string) string {
	parts := strings.Split(version, "-")
	if len(parts) < 3 {
		return ""
	}
	commit := parts[len(parts)-1]
	if len(commit) != 12 {
		return ""
	}
	for _, c := range commit {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return ""
		}
	}
	return commit
}

var (
	// GitCommit is set via -ldflags during build
	GitCommit = "unknown"
	// Version is the semantic version
	Version = "dev"
)

// applyBuildInfoFallback fills in Version/GitCommit from the Go module's
// build info when they weren't set via -ldflags, e.g. for `go install
// .../rosactl@latest`, which doesn't run the Makefile.
func applyBuildInfoFallback() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if Version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		Version = info.Main.Version
	}
	if GitCommit == "unknown" {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				commit := s.Value
				if len(commit) > 12 {
					commit = commit[:12]
				}
				GitCommit = commit
			}
		}
		// go install .../rosactl@latest downloads a module zip with no VCS
		// metadata, so vcs.revision above won't be set; fall back to the
		// commit hash embedded in the module's pseudo-version instead.
		if GitCommit == "unknown" {
			if commit := commitFromPseudoVersion(info.Main.Version); commit != "" {
				GitCommit = commit
			}
		}
	}
}

func NewVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Long:  "Display the version and git commit hash of the rosactl binary.",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			applyBuildInfoFallback()
			fmt.Printf("rosactl version %s (commit: %s)\n", Version, GitCommit)
		},
	}
	return cmd
}
