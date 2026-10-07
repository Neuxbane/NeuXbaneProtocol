package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func runDownload(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: nxp download <target> [outfile]\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  nxp download @api1/storage/download?id=file.csv ./local.csv\n")
		fmt.Fprintf(os.Stderr, "  nxp download @api1/storage/download?id=file.csv | grep 'important'\n")
		fmt.Fprintf(os.Stderr, "  nxp download @api1/storage/download?id=file.csv | nxp upload @api2/storage/upload\n")
		os.Exit(1)
	}

	rawTarget := args[0]
	targetInfo, err := ResolveTarget(rawTarget)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	req, err := http.NewRequest("GET", targetInfo.ResolvedURL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if targetInfo.Token != "" {
		req.Header.Set("Authorization", "Bearer "+targetInfo.Token)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: download failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "Error: download returned HTTP %d: %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))
		os.Exit(1)
	}

	var outWriter io.Writer = os.Stdout

	if len(args) > 1 && args[1] != "-" {
		outFile := args[1]
		f, err := os.Create(outFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: create output file %s: %v\n", outFile, err)
			os.Exit(1)
		}
		defer f.Close()
		outWriter = f
	}

	_, err = io.Copy(outWriter, resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: stream download failed: %v\n", err)
		os.Exit(1)
	}
}

