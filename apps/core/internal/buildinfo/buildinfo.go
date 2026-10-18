// Package buildinfo carries the version stamped into the binaries at build time.
package buildinfo

// Version is set with -ldflags "-X orion/core/internal/buildinfo.Version=...". It is "dev" for a plain go build.
var Version = "dev"
