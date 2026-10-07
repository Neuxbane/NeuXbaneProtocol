package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/files"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "connect":
		runConnect(args)
	case "endpoints", "ls":
		runEndpoints(args)
	case "use":
		runUse(args)
	case "call", "req":
		runCall(args)
	case "upload":
		runUpload(args)
	case "download":
		runDownload(args)
	case "inspect":
		runInspect(args)
	case "routes":
		runRoutes(args)
	case "schema":
		runSchema(args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stdout, `nxp - NeuXbaneProtocol Multi-API Orchestration & Piping CLI

Usage:
  nxp connect [name] <url> [--token <jwt>]   Connect and register a named API endpoint
  nxp endpoints                              List all registered API endpoints
  nxp use <name>                             Set active default API endpoint
  nxp call [METHOD] <target> [-d data]       Call an API, support piped stdin & stdout
  nxp upload <target> [file] [--url link]    Upload file (or stdin stream) to target API
  nxp download <target> [outfile]            Download file from API, stream to stdout
  nxp inspect <target> [--from N] [--len N]  Inspect byte-range from file, URL, or API
  nxp routes [endpoint]                      Display route tree for endpoint
  nxp schema [endpoint]                      Retrieve JSON Schema bundle
  nxp help                                   Show this help message

Target Addressing (Strict Standard):
  Target must strictly be one of:
    @<endpoint>/<path>   Addresses named endpoint (e.g., @api1/items, @api2/upload)
    /<path>              Addresses currently active endpoint (e.g., /items)
    https://...          Direct absolute URL
    ./file.ext           Local file path (inspect / upload)

Piping Examples:
  # 1. Connect multiple APIs:
  nxp connect api1 http://localhost:8080
  nxp connect api2 http://localhost:9090

  # 2. Download from API 1 and pipe directly to upload on API 2:
  nxp download @api1/storage/download?id=data.csv | nxp upload @api2/storage/upload

  # 3. Call API 1 and pipe JSON response to API 2:
  nxp call GET @api1/export | nxp call POST @api2/import

  # 4. Inspect 5KB byte range from API 1 and upload that slice to API 2:
  nxp inspect @api1/storage/download?id=huge.log --from 0 --length 5000 | nxp upload @api2/storage/upload

  # 5. Forward download URL directly without streaming bytes:
  nxp upload @api2/storage/upload --url "https://api1.com/files/report.pdf"
`)
}

func runConnect(args []string) {
	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	tokenFlag := fs.String("token", "", "Bearer token for authentication")

	var positional []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			if args[i] == "--token" && i+1 < len(args) {
				*tokenFlag = args[i+1]
				i++
			} else {
				_ = fs.Parse(args[i:])
				break
			}
		} else {
			positional = append(positional, args[i])
		}
	}

	if len(positional) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: nxp connect [name] <url> [--token <jwt>]\n")
		os.Exit(1)
	}

	name := "default"
	rawURL := ""

	if len(positional) == 1 {
		rawURL = positional[0]
	} else {
		name = positional[0]
		rawURL = positional[1]
	}

	rawURL = strings.TrimRight(rawURL, "/")
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "http://" + rawURL
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("GET", rawURL+"/?nxp=json", nil)
	if *tokenFlag != "" {
		req.Header.Set("Authorization", "Bearer "+*tokenFlag)
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to verify %s: %v (saving anyway)\n", rawURL, err)
	} else {
		resp.Body.Close()
	}

	cfg := loadConfig()
	cfg.Endpoints[name] = Endpoint{
		Name:  name,
		URL:   rawURL,
		Token: *tokenFlag,
	}
	cfg.Current = name

	if err := saveConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not save config: %v\n", err)
	}

	fmt.Printf("✓ Registered endpoint %q -> %s (active)\n", name, rawURL)
}

func runEndpoints(args []string) {
	cfg := loadConfig()
	if len(cfg.Endpoints) == 0 {
		fmt.Println("No endpoints registered. Use 'nxp connect <name> <url>' to add one.")
		return
	}

	fmt.Printf("%-3s %-15s %s\n", "ACT", "NAME", "URL")
	fmt.Printf("%-3s %-15s %s\n", "---", "----", "---")
	for name, ep := range cfg.Endpoints {
		act := " "
		if name == cfg.Current {
			act = "*"
		}
		fmt.Printf(" %s  %-15s %s\n", act, name, ep.URL)
	}
}

func runUse(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: nxp use <endpoint-name>\n")
		os.Exit(1)
	}
	name := args[0]
	cfg := loadConfig()
	if _, ok := cfg.Endpoints[name]; !ok {
		fmt.Fprintf(os.Stderr, "Error: endpoint %q not found. Registered: %v\n", name, listEndpointNames(cfg))
		os.Exit(1)
	}
	cfg.Current = name
	_ = saveConfig(cfg)
	fmt.Printf("✓ Switched active endpoint to %q\n", name)
}

func runInspect(args []string) {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	fromFlag := fs.Int64("from", 0, "Starting byte offset")
	lenFlag := fs.Int64("length", 1024, "Byte length to inspect")
	rawFlag := fs.Bool("raw", false, "Output raw bytes directly to stdout")
	jsonFlag := fs.Bool("json", false, "Output JSON metadata")

	var target string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				if arg != "--raw" && arg != "-raw" && arg != "--json" && arg != "-json" {
					i++
					flagArgs = append(flagArgs, args[i])
				}
			}
		} else if target == "" {
			target = arg
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		os.Exit(1)
	}

	if target == "" {
		fmt.Fprintf(os.Stderr, "Error: missing target file, URL, or endpoint to inspect\nUsage: nxp inspect <target> [--from N] [--length N] [--raw]\n")
		os.Exit(1)
	}

	targetInfo, err := ResolveTarget(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	result, err := files.Inspect(ctx, targetInfo.ResolvedURL, *fromFlag, *lenFlag, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: inspect failed: %v\n", err)
		os.Exit(1)
	}

	shouldStreamRaw := *rawFlag || (!*jsonFlag && !isTerminal(os.Stdout))

	if *jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return
	}

	if shouldStreamRaw {
		_, _ = os.Stdout.Write(result.Data)
		return
	}

	// Interactive terminal display
	fmt.Printf("--- NXP Inspect ---\n")
	fmt.Printf("Source:     %s\n", result.Source)
	fmt.Printf("Range:      bytes %d-%d\n", result.From, result.From+result.ReadBytes)
	fmt.Printf("Read:       %d bytes\n", result.ReadBytes)
	if result.TotalSize > 0 {
		fmt.Printf("Total:      %d bytes (%.1f KB)\n", result.TotalSize, float64(result.TotalSize)/1024.0)
	}
	fmt.Printf("EOF:        %v\n", result.EOF)
	fmt.Printf("Type:       %s\n", map[bool]string{true: "Text (utf-8)", false: "Binary"}[result.IsText])
	fmt.Printf("-------------------\n\n")

	if result.IsText {
		fmt.Println(result.DataText)
	} else {
		fmt.Println(result.DataHex)
	}
}

func runRoutes(args []string) {
	target := "/"
	if len(args) > 0 {
		target = args[0]
	}

	targetInfo, err := ResolveTarget(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	reqURL, _ := url.Parse(targetInfo.ResolvedURL + "?nxp=tree")
	req, _ := http.NewRequest("GET", reqURL.String(), nil)
	if targetInfo.Token != "" {
		req.Header.Set("Authorization", "Bearer "+targetInfo.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to fetch routes: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Error: server returned status %d\n", resp.StatusCode)
		os.Exit(1)
	}

	_, _ = io.Copy(os.Stdout, resp.Body)
}

func runSchema(args []string) {
	target := "/__nxp/schema"
	if len(args) > 0 {
		target = args[0]
		if !strings.Contains(target, "__nxp/schema") {
			target = strings.TrimRight(target, "/") + "/__nxp/schema"
		}
	}

	targetInfo, err := ResolveTarget(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	req, _ := http.NewRequest("GET", targetInfo.ResolvedURL, nil)
	if targetInfo.Token != "" {
		req.Header.Set("Authorization", "Bearer "+targetInfo.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to fetch schema: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Error: server returned status %d\n", resp.StatusCode)
		os.Exit(1)
	}

	var obj any
	if err := json.NewDecoder(resp.Body).Decode(&obj); err == nil {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(obj)
	} else {
		_, _ = io.Copy(os.Stdout, resp.Body)
	}
}
