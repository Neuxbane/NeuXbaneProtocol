package main

import (
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type fieldList []string

func (f *fieldList) String() string {
	return strings.Join(*f, ", ")
}

func (f *fieldList) Set(v string) error {
	*f = append(*f, v)
	return nil
}

func runUpload(args []string) {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)
	fieldName := fs.String("name", "file", "Form field name for file upload")
	fileName := fs.String("filename", "", "Uploaded file name override")
	forwardURL := fs.String("url", "", "Forward a remote download URL instead of file bytes")
	var extraFields fieldList
	fs.Var(&extraFields, "field", "Additional form field (key=val)")

	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			if arg == "--name" && i+1 < len(args) {
				*fieldName = args[i+1]
				i++
			} else if arg == "--filename" && i+1 < len(args) {
				*fileName = args[i+1]
				i++
			} else if arg == "--url" && i+1 < len(args) {
				*forwardURL = args[i+1]
				i++
			} else if arg == "--field" && i+1 < len(args) {
				_ = extraFields.Set(args[i+1])
				i++
			} else {
				_ = fs.Parse(args[i:])
				break
			}
		} else {
			positional = append(positional, arg)
		}
	}

	if len(positional) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: nxp upload <target> [file] [--url <link>] [--field k=v]\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  nxp upload @api2/storage/upload ./photo.png\n")
		fmt.Fprintf(os.Stderr, "  nxp download @api1/storage/download?id=123 | nxp upload @api2/storage/upload\n")
		fmt.Fprintf(os.Stderr, "  nxp upload @api2/storage/upload --url https://api1.com/files/report.pdf\n")
		os.Exit(1)
	}

	rawTarget := positional[0]
	targetInfo, err := ResolveTarget(rawTarget)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// 1. If --url is provided, forward the URL reference directly
	if *forwardURL != "" {
		jsonBody := fmt.Sprintf(`{"url":%q}`, *forwardURL)
		req, err := http.NewRequest("POST", targetInfo.ResolvedURL, strings.NewReader(jsonBody))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		req.Header.Set("Content-Type", "application/json")
		if targetInfo.Token != "" {
			req.Header.Set("Authorization", "Bearer "+targetInfo.Token)
		}
		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: upload forward failed: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		_, _ = io.Copy(os.Stdout, resp.Body)
		return
	}

	// 2. Stream file from argument or Stdin
	var sourceReader io.Reader
	uploadFilename := "upload.bin"

	if len(positional) > 1 && positional[1] != "-" {
		filePath := positional[1]
		f, err := os.Open(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot open file %s: %v\n", filePath, err)
			os.Exit(1)
		}
		defer f.Close()
		sourceReader = f
		uploadFilename = filepath.Base(filePath)
	} else if !isTerminal(os.Stdin) {
		sourceReader = os.Stdin
	} else {
		fmt.Fprintf(os.Stderr, "Error: no input file provided and stdin is empty\n")
		os.Exit(1)
	}

	if *fileName != "" {
		uploadFilename = *fileName
	}

	// Build multipart streaming body
	pipeReader, pipeWriter := io.Pipe()
	mpWriter := multipart.NewWriter(pipeWriter)

	go func() {
		defer pipeWriter.Close()
		defer mpWriter.Close()

		// Write extra form fields
		for _, f := range extraFields {
			parts := strings.SplitN(f, "=", 2)
			if len(parts) == 2 {
				_ = mpWriter.WriteField(parts[0], parts[1])
			}
		}

		// Write file field header
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, *fieldName, uploadFilename))
		h.Set("Content-Type", "application/octet-stream")

		part, err := mpWriter.CreatePart(h)
		if err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}

		if _, err := io.Copy(part, sourceReader); err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
	}()

	req, err := http.NewRequest("POST", targetInfo.ResolvedURL, pipeReader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", mpWriter.FormDataContentType())
	if targetInfo.Token != "" {
		req.Header.Set("Authorization", "Bearer "+targetInfo.Token)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: upload to %s failed: %v\n", targetInfo.ResolvedURL, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && isTerminal(os.Stdout) {
		fmt.Fprintf(os.Stderr, "[HTTP %d %s]\n", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	_, _ = io.Copy(os.Stdout, resp.Body)
	if isTerminal(os.Stdout) {
		fmt.Fprintln(os.Stdout)
	}
}
