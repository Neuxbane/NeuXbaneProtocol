package router

import (
	"strings"
)

type pathSegment struct {
	literal   string
	paramName string
	isParam   bool
	isWild    bool
}

type pathPattern struct {
	raw       string
	segments  []pathSegment
	isDynamic bool
}

func parsePattern(path string) *pathPattern {
	clean := cleanRoutePath(path)
	parts := strings.Split(strings.Trim(clean, "/"), "/")
	if len(parts) == 1 && parts[0] == "" {
		parts = []string{}
	}

	p := &pathPattern{
		raw:      clean,
		segments: make([]pathSegment, len(parts)),
	}

	for i, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			p.isDynamic = true
			p.segments[i] = pathSegment{
				paramName: part[1 : len(part)-1],
				isParam:   true,
			}
		} else if strings.HasPrefix(part, "[") && strings.HasSuffix(part, "]") {
			p.isDynamic = true
			p.segments[i] = pathSegment{
				paramName: part[1 : len(part)-1],
				isParam:   true,
			}
		} else if part == "*" || part == "+" {
			p.isDynamic = true
			p.segments[i] = pathSegment{
				isWild: true,
			}
		} else {
			p.segments[i] = pathSegment{
				literal: part,
			}
		}
	}

	return p
}

func (p *pathPattern) match(path string) (map[string]string, bool) {
	clean := cleanRoutePath(path)
	parts := strings.Split(strings.Trim(clean, "/"), "/")
	if len(parts) == 1 && parts[0] == "" {
		parts = []string{}
	}

	if len(parts) != len(p.segments) {
		return nil, false
	}

	params := make(map[string]string)
	for i, seg := range p.segments {
		reqPart := parts[i]
		if seg.isParam {
			params[seg.paramName] = reqPart
		} else if seg.isWild {
			continue
		} else if seg.literal != reqPart {
			return nil, false
		}
	}

	return params, true
}
