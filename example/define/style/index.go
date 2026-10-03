package style

import "abi/rest"

func Handler(ctx *rest.Ctx) (string, error) {
	return ctx.CSS(`
body {
  margin: 0;
  font-family: system-ui, sans-serif;
  background-color: #0b0f19;
  color: #f8fafc;
}
`)
}
