package introspect

import (
	"fmt"
	"html"
	"strings"
)

// RenderHTML generates a modern HTML page for browser-based API introspection.
func RenderHTML(view *Response) string {
	var sb strings.Builder

	sb.WriteString("<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>nxp API - ")
	sb.WriteString(html.EscapeString(view.Path))
	sb.WriteString("</title><style>")
	sb.WriteString("body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif;margin:40px;background:#f8fafc;color:#1e293b;}")
	sb.WriteString(".card{background:#fff;border-radius:8px;padding:24px;box-shadow:0 1px 3px rgba(0,0,0,0.1);max-width:800px;margin:0 auto;}")
	sb.WriteString("h1{font-size:24px;margin-top:0;color:#0f172a;border-bottom:1px solid #e2e8f0;padding-bottom:12px;}")
	sb.WriteString(".badge{display:inline-block;padding:2px 8px;border-radius:4px;font-size:12px;font-weight:600;margin-right:6px;background:#e2e8f0;color:#334155;}")
	sb.WriteString(".badge-get{background:#dcfce7;color:#15803d;}")
	sb.WriteString(".badge-post{background:#dbeafe;color:#1d4ed8;}")
	sb.WriteString("ul{list-style:none;padding:0;margin:16px 0;}")
	sb.WriteString("li{padding:10px 12px;border-bottom:1px solid #f1f5f9;display:flex;align-items:center;}")
	sb.WriteString("li:last-child{border-bottom:none;}")
	sb.WriteString("a{color:#2563eb;text-decoration:none;font-weight:500;margin-right:12px;}")
	sb.WriteString("a:hover{text-decoration:underline;}")
	sb.WriteString("</style></head><body><div class=\"card\">")

	sb.WriteString("<h1>Path: ")
	sb.WriteString(html.EscapeString(view.Path))
	sb.WriteString("</h1>")

	if view.Self != nil {
		sb.WriteString("<h3>Self Handler: ")
		sb.WriteString(html.EscapeString(view.Self.Handler))
		sb.WriteString("</h3><div>")
		for _, m := range view.Self.Methods {
			cls := "badge"
			if m == "GET" {
				cls += " badge-get"
			} else if m == "POST" {
				cls += " badge-post"
			}
			sb.WriteString(fmt.Sprintf("<span class=\"%s\">%s</span>", cls, m))
		}
		if view.Self.Auth != "" {
			sb.WriteString(fmt.Sprintf("<span class=\"badge\">auth: %s</span>", view.Self.Auth))
		}
		sb.WriteString("</div>")
	}

	sb.WriteString("<h3>Children:</h3><ul>")
	if len(view.Children) == 0 {
		sb.WriteString("<li><em>No sub-routes at this level</em></li>")
	} else {
		for _, c := range view.Children {
			target := view.Path
			if target == "/" {
				target = ""
			}
			target = target + "/" + c.Name + "?nxp"

			sb.WriteString("<li><a href=\"")
			sb.WriteString(html.EscapeString(target))
			sb.WriteString("\">")
			sb.WriteString(html.EscapeString(c.Name))
			if c.Kind == "branch" {
				sb.WriteString("/")
			}
			sb.WriteString("</a>")

			for _, m := range c.Methods {
				sb.WriteString(fmt.Sprintf("<span class=\"badge\">%s</span>", m))
			}
			if c.Dynamic {
				sb.WriteString("<span class=\"badge\">dynamic</span>")
			}
			sb.WriteString("</li>")
		}
	}
	sb.WriteString("</ul></div></body></html>")

	return sb.String()
}
