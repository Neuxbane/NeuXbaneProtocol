// Package config answers: what settings, addresses, limits, and flags govern this runtime instance?
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"
)

// Environment specifies the deployment tier.
type Environment string

const (
	EnvDev     Environment = "dev"
	EnvStaging Environment = "staging"
	EnvProd    Environment = "prod"
)

// EgressPolicy dictates action taken when a worker response violates its contract shape.
type EgressPolicy string

const (
	EgressPolicyWarn       EgressPolicy = "warn"
	EgressPolicyQuarantine EgressPolicy = "quarantine"
	EgressPolicyKill       EgressPolicy = "kill"
)

// RateLimit holds rps and burst values.
type RateLimit struct {
	RPS   int `json:"rps"`
	Burst int `json:"burst"`
}

// IntrospectConfig controls the ?nxp introspection system.
type IntrospectConfig struct {
	Enabled      bool      `json:"enabled"`
	Public       bool      `json:"public"`
	PublicSchema bool      `json:"publicSchema"`
	Redact       []string  `json:"redact"`
	RateLimit    RateLimit `json:"rateLimit"`
	HTML         bool      `json:"html"`
	BundlePath   string    `json:"bundlePath"`
}

// Config encompasses all configuration parameters for the mother process.
type Config struct {
	Env          Environment      `json:"env"`
	Addr         string           `json:"addr"`          // e.g. ":8080"
	SocketDir    string           `json:"socket_dir"`    // IPC socket directory
	DefineDir    string           `json:"define_dir"`    // path to define/ directory
	EgressPolicy EgressPolicy     `json:"egress_policy"` // default "quarantine"
	Debounce     time.Duration    `json:"debounce"`      // watcher debounce, default 250ms
	Introspect   IntrospectConfig `json:"introspect"`
}

// DefaultConfig generates a Config instance with development defaults.
func DefaultConfig() *Config {
	envStr := os.Getenv("NXP_ENV")
	env := EnvDev
	if envStr == "prod" || envStr == "production" {
		env = EnvProd
	} else if envStr == "staging" {
		env = EnvStaging
	}

	isDev := (env == EnvDev)

	sockDir := os.Getenv("NXP_SOCK_DIR")
	if sockDir == "" {
		// Scope the socket directory per application instance so that multiple
		// nxp servers running on the same host never share (and therefore never
		// delete) each other's worker binaries or sockets on shutdown.
		base := filepath.Join(os.TempDir(), "nxp-sockets")
		sockDir = filepath.Join(base, instanceKey())
	}

	addr := os.Getenv("NXP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	defineDir := os.Getenv("NXP_DEFINE_DIR")
	if defineDir == "" {
		defineDir = "define"
	}

	return &Config{
		Env:          env,
		Addr:         addr,
		SocketDir:    sockDir,
		DefineDir:    defineDir,
		EgressPolicy: EgressPolicyQuarantine,
		Debounce:     250 * time.Millisecond,
		Introspect: IntrospectConfig{
			Enabled:      true,
			Public:       isDev,
			PublicSchema: isDev,
			Redact:       []string{"x-internal-token", "internal-id", "authorization"},
			RateLimit: RateLimit{
				RPS:   100,
				Burst: 200,
			},
			HTML:       true,
			BundlePath: "/__nxp/schema",
		},
	}
}

// instanceKey derives a stable, filesystem-safe identifier for the current
// application instance from its absolute working directory. It is used to give
// each nxp server its own socket subdirectory so that concurrent servers on the
// same host do not clobber one another's worker artifacts.
func instanceKey() string {
	wd, err := os.Getwd()
	if err != nil || wd == "" {
		wd = "default"
	}
	if abs, err := filepath.Abs(wd); err == nil {
		wd = abs
	}
	sum := sha256.Sum256([]byte(wd))
	return hex.EncodeToString(sum[:8])
}
