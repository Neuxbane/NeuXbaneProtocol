// @guard
// @desc Download a stored file
package get

import (
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/errors"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/files"
)

// Handler handles file downloads with Range and ETag support.
func Handler(ctx *files.DownloadCtx) (files.DownloadResult, error) {
	fileID := ctx.Param("id")
	if fileID == "" {
		fileID = "default.dat"
	}
	if fileID == "forbidden" {
		return files.DownloadResult{}, errors.New(errors.CodeForbidden, "access denied", 403)
	}

	return files.DownloadResult{
		Source:    fileID,
		Name:      "downloaded_" + fileID,
		Presigned: false,
	}, nil
}
