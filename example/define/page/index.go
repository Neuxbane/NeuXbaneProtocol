package page

import "abi/rest"

func Handler(ctx *rest.Ctx) (string, error) {
	return ctx.HTML(`<!DOCTYPE html>
<html>
<head><title>NeuXbane Protocol</title></head>
<body>
  <h1>Served from nxp!</h1>
  <p>Raw HTML rendered directly by Go backend!</p>
</body>
</html>`)
}
