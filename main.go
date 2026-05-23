package main

import (
	"github.com/vtech-com/capigo-api-sdk/cmd"
	"github.com/vtech-com/capigo-api-sdk/internal/version"
)

// These variables are set by ldflags at build time:
//
//	-X main.version=v0.1.0
//	-X main.commit=abc1234
//	-X main.date=2026-01-01T00:00:00Z
var (
	buildVersion = "dev"
	buildCommit  = "none"
	buildDate    = "unknown"
)

func main() {
	version.Version = buildVersion
	version.Commit = buildCommit
	version.Date = buildDate
	cmd.Execute()
}
