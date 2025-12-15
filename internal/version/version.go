package version

// Version information - injected at build time via ldflags
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// GetVersion returns the full version string
func GetVersion() string {
	return Version
}

// GetFullVersion returns the version with commit and build date
func GetFullVersion() string {
	return Version + " (commit: " + Commit + ", built: " + Date + ")"
}
