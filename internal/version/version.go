package version

// Version information
var (
	Version = "0.2.3"
	Commit = "3e76c5e"
	Date = "2025-05-05 23:04:49 UTC"
)

// GetVersion returns the full version string
func GetVersion() string {
	return Version
}

// GetFullVersion returns the version with commit and build date
func GetFullVersion() string {
	return Version + " (commit: " + Commit + ", built: " + Date + ")"
}
