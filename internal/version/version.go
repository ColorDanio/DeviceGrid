package version

// Version information — injected at build time via ldflags
// Format: YYYY.M.D_commitSHA
var (
	Version   = "dev"
	BuildDate = "unknown"
	GitCommit = "unknown"
)
