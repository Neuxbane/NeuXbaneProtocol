// Package codegen answers: how is the define/ directory tree compiled into routes, contracts, and execution adapters?
package codegen

import (
	"bufio"
	"strconv"
	"strings"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// Directives represents comment metadata annotations extracted from a handler file.
type Directives struct {
	Route       string
	Transport   abi.Transport
	Method      string
	Auth        string
	Scopes      []string
	RateLimit   *abi.RateLimitConfig
	Stream      bool
	Topic       string
	QoS         int
	Retain      bool
	Group       string
	Partitions  int
}

// ParseDirectives extracts @directives from the initial comment block of a Go file.
func ParseDirectives(content string) *Directives {
	d := &Directives{
		Scopes: make([]string, 0),
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "/*") && !strings.HasPrefix(line, "*") {
			// Once code or package clause begins, stop scanning comment block
			if strings.HasPrefix(line, "package ") {
				break
			}
			continue
		}

		comment := strings.TrimLeft(line, "/* \t")
		if !strings.HasPrefix(comment, "@") {
			continue
		}

		parts := strings.Fields(comment)
		if len(parts) == 0 {
			continue
		}

		directive := parts[0]
		args := parts[1:]

		switch directive {
		case "@route":
			if len(args) > 0 {
				d.Route = args[0]
			}
		case "@transport":
			if len(args) > 0 {
				tr := abi.Transport(strings.ToLower(args[0]))
				if tr.IsValid() {
					d.Transport = tr
				}
			}
		case "@method":
			if len(args) > 0 {
				d.Method = strings.ToUpper(args[0])
			}
		case "@auth":
			if len(args) > 0 {
				d.Auth = strings.ToLower(args[0]) // "required", "optional", "public"
			}
		case "@scope", "@scopes":
			d.Scopes = append(d.Scopes, args...)
		case "@ratelimit":
			if len(args) >= 2 {
				rps, _ := strconv.Atoi(args[0])
				burst, _ := strconv.Atoi(args[1])
				d.RateLimit = &abi.RateLimitConfig{RPS: rps, Burst: burst}
			}
		case "@stream":
			d.Stream = true
		case "@topic":
			if len(args) > 0 {
				d.Topic = args[0]
			}
		case "@qos":
			if len(args) > 0 {
				d.QoS, _ = strconv.Atoi(args[0])
			}
		case "@retain":
			d.Retain = true
		case "@group":
			if len(args) > 0 {
				d.Group = args[0]
			}
		case "@partitions":
			if len(args) > 0 {
				d.Partitions, _ = strconv.Atoi(args[0])
			}
		}
	}

	return d
}
