package avatar

import "abi/rest"

func Handler(ctx *rest.Ctx) (any, error) {
	// SVG image generated dynamically in Go or retrieved from a database
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100">
  <circle cx="50" cy="50" r="40" stroke="green" stroke-width="4" fill="yellow" />
</svg>`
	return ctx.Blob("image/svg+xml", []byte(svg))
}
