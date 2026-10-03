package introspect

import (
	"fmt"
	"strings"
)

// RenderTree generates a Unix-style tree view of the current path and its immediate children.
func RenderTree(view *Response) string {
	var sb strings.Builder

	sb.WriteString(view.Path)
	if view.Self != nil {
		sb.WriteString(fmt.Sprintf(" [%s]", strings.Join(view.Self.Methods, ", ")))
		if view.Self.Auth != "" {
			sb.WriteString(fmt.Sprintf(" (auth: %s)", view.Self.Auth))
		}
	}
	sb.WriteString("\n")

	total := len(view.Children)
	for i, c := range view.Children {
		prefix := "├── "
		if i == total-1 {
			prefix = "└── "
		}

		name := c.Name
		if c.Kind == "branch" {
			name += "/"
		}

		sb.WriteString(prefix)
		sb.WriteString(name)

		if len(c.Methods) > 0 {
			sb.WriteString(fmt.Sprintf(" [%s]", strings.Join(c.Methods, ", ")))
		}
		if c.Auth != "" {
			sb.WriteString(fmt.Sprintf(" (auth: %s)", c.Auth))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
