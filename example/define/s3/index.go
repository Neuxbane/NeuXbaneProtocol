package s3

import (
	"abi/files"
	"abi/rest"
)

func Handler(ctx *rest.Ctx) (files.DownloadResult, error) {
	return files.DownloadResult{
		Source:    "https://example-bucket.s3.amazonaws.com/files/dataset.parquet?AWSAccessKeyId=AKIAIOSFODNN7EXAMPLE",
		Name:      "dataset.parquet",
		Presigned: true,
	}, nil
}
