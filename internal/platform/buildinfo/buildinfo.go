package buildinfo

// These variables are populated by go build -ldflags in release builds.
var (
	Version   = "dev"
	GitSHA    = "unknown"
	BuildTime = "unknown"
)

type Info struct {
	Version   string `json:"version,omitempty"`
	GitSHA    string `json:"git_sha,omitempty"`
	BuildTime string `json:"build_time,omitempty"`
}

func Current() Info {
	return Info{
		Version:   Version,
		GitSHA:    GitSHA,
		BuildTime: BuildTime,
	}
}
