package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type headerFlags []string

func (h *headerFlags) String() string {
	return strings.Join(*h, ", ")
}

func (h *headerFlags) Set(v string) error {
	*h = append(*h, v)
	return nil
}

func runCall(args []string) {
	fs := flag.NewFlagSet("call", flag.ExitOnError)
	methodFlag := fs.String("X", "", "HTTP method (GET, POST, PUT, DELETE, PATCH)")
	dataFlag := fs.String("d", "", "HTTP request body data (string, or @filename)")
	rawFlag := fs.Bool("raw", false, "Always output raw response bytes")
	var customHeaders headerFlags
	fs.Var(&customHeaders, "H", "Custom HTTP header (key: value)")

	// Parse flags and extract positional arguments
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			// check known flags
			if arg == "-X" && i+1 < len(args) {
				*methodFlag = args[i+1]
				i++
			} else if arg == "-d" && i+1 < len(args) {
				*dataFlag = args[i+1]
				i++
			} else if arg == "-H" && i+1 < len(args) {
				_ = customHeaders.Set(args[i+1])
				i++
			} else if arg == "--raw" || arg == "-raw" {
				*rawFlag = true
			} else {
				// parse via FlagSet
				_ = fs.Parse(args[i:])
				break
			}
		} else {
			positional = append(positional, arg)
		}
	}

	if len(positional) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: nxp call [METHOD] <target> [-d data] [-H header]\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  nxp call GET @api1/items\n")
		fmt.Fprintf(os.Stderr, "  nxp call POST @api2/process -d '{\"id\": 123}'\n")
		fmt.Fprintf(os.Stderr, "  nxp call GET @api1/export | nxp call POST @api2/import\n")
		os.Exit(1)
	}

	method := "GET"
	var rawTarget string

	first := strings.ToUpper(positional[0])
	if first == "GET" || first == "POST" || first == "PUT" || first == "DELETE" || first == "PATCH" || first == "HEAD" || first == "OPTIONS" {
		method = first
		if len(positional) > 1 {
			rawTarget = positional[1]
		}
	} else {
		rawTarget = positional[0]
	}

	if rawTarget == "" {
		fmt.Fprintf(os.Stderr, "Error: missing target (e.g. '@api1/items' or 'https://...')\n")
		os.Exit(1)
	}

	if *methodFlag != "" {
		method = strings.ToUpper(*methodFlag)
	}

	// Prepare request body
	var bodyReader io.Reader
	var hasBody bool

	if *dataFlag != "" {
		hasBody = true
		if strings.HasPrefix(*dataFlag, "@") {
			filePath := strings.TrimPrefix(*dataFlag, "@")
			f, err := os.Open(filePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: cannot open data file %s: %v\n", filePath, err)
				os.Exit(1)
			}
			defer f.Close()
			b, _ := io.ReadAll(f)
			bodyReader = bytes.NewReader(b)
		} else {
			bodyReader = strings.NewReader(*dataFlag)
		}
	} else if !isTerminal(os.Stdin) {
		// Read piped stdin as body!
		stdinBytes, err := io.ReadAll(os.Stdin)
		if err == nil && len(stdinBytes) > 0 {
			hasBody = true
			bodyReader = bytes.NewReader(stdinBytes)
			if method == "GET" && *methodFlag == "" {
				method = "POST"
			}
		}
	}

	targetInfo, err := ResolveTarget(rawTarget)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	req, err := http.NewRequest(method, targetInfo.ResolvedURL, bodyReader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: build request failed: %v\n", err)
		os.Exit(1)
	}

	if targetInfo.Token != "" {
		req.Header.Set("Authorization", "Bearer "+targetInfo.Token)
	}

	// Apply custom headers
	for _, h := range customHeaders {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}

	// Auto detect JSON if Content-Type not specified
	if hasBody && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: request to %s failed: %v\n", targetInfo.ResolvedURL, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && !*rawFlag && isTerminal(os.Stdout) {
		fmt.Fprintf(os.Stderr, "[HTTP %d %s]\n", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	// Stream body directly to stdout
	_, _ = io.Copy(os.Stdout, resp.Body)

	if resp.StatusCode >= 400 && isTerminal(os.Stdout) {
		fmt.Fprintln(os.Stdout)
	}
}

