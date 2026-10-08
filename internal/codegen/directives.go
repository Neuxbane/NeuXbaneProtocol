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
	Transport         abi.Transport
	Method            string
	Auth              string
	Scopes            []string
	Guards            []string
	HasGuardDirective bool
	RateLimit         *abi.RateLimitConfig
	Shape             string
	Stream            bool
	Topic             string
	QoS               int
	Retain            bool
	Group             string
	Partitions        int
	Description       string

	// StreamContract holds the declarative poll contract for a live stream
	// route (see @stream). It is emitted into the route metadata so generic
	// consumers can drive the stream without endpoint-specific code.
	StreamContract map[string]string
}

// ParseDirectives extracts @directives from the initial comment block of a Go file.
func ParseDirectives(content string) *Directives {
	d := &Directives{
		Scopes: make([]string, 0),
		Guards: make([]string, 0),
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
		case "@guard", "@guards":
			d.HasGuardDirective = true
			// Guard names are space- or comma-separated. A name may itself
			// contain a slash (e.g. "user/admin") to reference a nested guard.
			// Bare "@guard" (no args) or "@guard none" indicates no guards (public).
			for _, arg := range args {
				for _, g := range strings.Split(arg, ",") {
					g = strings.TrimSpace(g)
					if g != "" && g != "none" {
						d.Guards = append(d.Guards, g)
					}
				}
			}
		case "@ratelimit":
			if len(args) >= 2 {
				rps, _ := strconv.Atoi(args[0])
				burst, _ := strconv.Atoi(args[1])
				d.RateLimit = &abi.RateLimitConfig{RPS: rps, Burst: burst}
			}
		case "@shape":
			if len(args) > 0 {
				d.Shape = strings.ToLower(args[0])
				if d.Shape == "stream" {
					d.Stream = true
				}
			}
		case "@stream":
			d.Stream = true
			// A stream route may declare a declarative poll contract as
			// key=value pairs, e.g.:
			//
			//   // @stream action=poll cursor=offset messages=updates text=message.text id=update_id
			//
			// The contract is transport-agnostic: it tells a generic consumer
			// how to build a poll request, where to find the message list in
			// the response, which field is the monotonic cursor, and which
			// field carries the human-readable text. Consumers (e.g. XiaoMAO's
			// stream poller) read it from the route metadata and drive any
			// stream endpoint without endpoint-specific code.
			for _, arg := range args {
				k, v, ok := strings.Cut(arg, "=")
				if !ok {
					continue
				}
				k = strings.TrimSpace(k)
				v = strings.TrimSpace(v)
				if k == "" || v == "" {
					continue
				}
				if d.StreamContract == nil {
					d.StreamContract = make(map[string]string)
				}
				d.StreamContract[k] = v
			}
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
		case "@desc", "@description":
			// The description is free-form text: capture everything after the
			// directive keyword, preserving internal spacing.
			if rest := strings.TrimSpace(strings.TrimPrefix(comment, directive)); rest != "" {
				d.Description = rest
			}
		}
	}

	return d
}
