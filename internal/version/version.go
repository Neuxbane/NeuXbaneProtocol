// Package version answers: what are the framework, ABI, and build versions of this binary?
package version

import (
	"fmt"
	"runtime"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

var (
	// FrameworkVersion is the release version of the nxp framework.
	FrameworkVersion = "0.1.0"

	// GitCommit is set via -ldflags during build.
	GitCommit = "dev"

	// BuildDate is set via -ldflags during build.
	BuildDate = "unknown"
)

// Info returns the aggregated version string.
func Info() string {
	return fmt.Sprintf("nxp %s (abi %s, commit %s, built %s, %s/%s)",
		FrameworkVersion, abi.ABIVersion, GitCommit, BuildDate, runtime.GOOS, runtime.GOARCH)
}
