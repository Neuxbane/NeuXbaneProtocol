package video

import (
	"path/filepath"

	"abi/files"
	"abi/rest"
)

func Handler(ctx *rest.Ctx) (files.DownloadResult, error) {
	absPath, _ := filepath.Abs("sample.mp4")
	return files.DownloadResult{
		Source: absPath,
		Name:   "sample.mp4",
		Mime:   "video/mp4",
	}, nil
}
