// @desc Upload a file to storage
package post

import (
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/errors"
	"github.com/Neuxbane/NeuXbaneProtocol/nxp/files"
)

// Handler handles file uploads with dedup and quarantine checks.
func Handler(ctx *files.UploadCtx) (files.UploadResult, error) {
	if ctx == nil {
		return files.UploadResult{
			ID:   "demo-upload-id",
			Size: 1024,
			Name: "demo_file.png",
		}, nil
	}

	// 1. Check quarantine
	if ctx.Quarantine() {
		_ = ctx.Discard()
		return files.UploadResult{}, errors.New(errors.CodeForbidden, "file quarantined due to executable content", 403)
	}

	// 2. Check dedup
	existingID, hit, _ := ctx.Dedupe()
	if hit {
		_ = ctx.Discard()
		return files.UploadResult{
			ID:   existingID,
			Size: ctx.Size(),
			Name: ctx.SafeName(),
		}, nil
	}

	// 3. Move to permanent storage
	destID := "upload-" + ctx.SafeName()
	_ = ctx.MoveTo(destID)

	return files.UploadResult{
		ID:   destID,
		Size: ctx.Size(),
		Name: ctx.SafeName(),
	}, nil
}
