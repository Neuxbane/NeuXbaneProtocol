package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Endpoint struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Token string `json:"token,omitempty"`
}

type Config struct {
	Current   string              `json:"current"`
	Endpoints map[string]Endpoint `json:"endpoints"`
}

func configPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".nxp", "config.json")
	}
	return filepath.Join(".", ".nxp", "config.json")
}

func loadConfig() Config {
	p := configPath()
	data, err := os.ReadFile(p)
	if err != nil {
		return Config{Endpoints: make(map[string]Endpoint)}
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{Endpoints: make(map[string]Endpoint)}
	}
	if cfg.Endpoints == nil {
		cfg.Endpoints = make(map[string]Endpoint)
	}
	return cfg
}

func saveConfig(cfg Config) error {
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// TargetInfo holds the resolved address and credentials for a target.
type TargetInfo struct {
	ResolvedURL string
	Token       string
	Endpoint    string
	IsLocalFile bool
}

// ResolveTarget parses targets according to strict addressing rules:
// - "@<endpoint>/<path>" (e.g. "@api1/storage/download")
// - "/<path>" (e.g. "/storage/download", routes to current active endpoint)
// - "http://..." or "https://..." (direct URL)
// - Local file path (if verified by filesystem)
func ResolveTarget(raw string) (TargetInfo, error) {
	cfg := loadConfig()

	// 1. Direct URL
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return TargetInfo{ResolvedURL: raw}, nil
	}

	// 2. Strict scoped endpoint syntax: @endpoint/path
	if strings.HasPrefix(raw, "@") {
		trimmed := strings.TrimPrefix(raw, "@")
		parts := strings.SplitN(trimmed, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return TargetInfo{}, fmt.Errorf("invalid endpoint target %q: must strictly follow '@<endpoint>/<path>' format (e.g., @api1/items)", raw)
		}

		epName := parts[0]
		relPath := "/" + parts[1]

		ep, ok := cfg.Endpoints[epName]
		if !ok {
			return TargetInfo{}, fmt.Errorf("unknown endpoint %q (registered endpoints: %v)", epName, listEndpointNames(cfg))
		}

		u, err := url.Parse(ep.URL)
		if err != nil {
			return TargetInfo{}, fmt.Errorf("invalid endpoint URL %q: %w", ep.URL, err)
		}

		joined := strings.TrimRight(u.String(), "/") + relPath
		return TargetInfo{
			ResolvedURL: joined,
			Token:       ep.Token,
			Endpoint:    epName,
		}, nil
	}

	// 3. Current active endpoint relative path: /path
	if strings.HasPrefix(raw, "/") {
		if cfg.Current == "" {
			return TargetInfo{}, fmt.Errorf("cannot resolve relative path %q: no active endpoint set. Run 'nxp connect <name> <url>' or 'nxp use <name>' first", raw)
		}

		ep, ok := cfg.Endpoints[cfg.Current]
		if !ok {
			return TargetInfo{}, fmt.Errorf("active endpoint %q not found in registry", cfg.Current)
		}

		u, err := url.Parse(ep.URL)
		if err != nil {
			return TargetInfo{}, fmt.Errorf("invalid endpoint URL %q: %w", ep.URL, err)
		}

		joined := strings.TrimRight(u.String(), "/") + raw
		return TargetInfo{
			ResolvedURL: joined,
			Token:       ep.Token,
			Endpoint:    cfg.Current,
		}, nil
	}

	// 4. Check if it's a local file
	if _, err := os.Stat(raw); err == nil {
		return TargetInfo{ResolvedURL: raw, IsLocalFile: true}, nil
	}

	// Reject any non-standard syntax strictly
	if strings.Contains(raw, ":/") {
		return TargetInfo{}, fmt.Errorf("unsupported syntax %q: NeuXbaneProtocol strictly enforces '@<endpoint>/<path>' format (e.g., @%s)", raw, strings.Replace(raw, ":/", "/", 1))
	}

	return TargetInfo{}, fmt.Errorf("invalid target %q: specify a '@<endpoint>/<path>' target, a '/<path>', or a direct URL", raw)
}

func listEndpointNames(cfg Config) []string {
	var names []string
	for k := range cfg.Endpoints {
		names = append(names, k)
	}
	return names
}

