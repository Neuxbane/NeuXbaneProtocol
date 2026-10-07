package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestCLIMultiAPIWorkflow(t *testing.T) {
	// API 1: Downloader & Exporter
	api1Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/export/json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"status":"ok","items":["alpha","beta"]}`)
			return
		}
		if r.URL.Path == "/download/report.csv" {
			w.Header().Set("Content-Type", "text/csv")
			_, _ = fmt.Fprint(w, "id,name,value\n1,alpha,100\n2,beta,200\n")
			return
		}
		if r.URL.Query().Get("nxp") == "json" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"path": "/", "name": "api1"})
			return
		}
		http.NotFound(w, r)
	}))
	defer api1Server.Close()

	// API 2: Ingestion & Uploader
	var receivedUploadName string
	var receivedUploadContent []byte
	var receivedCallBody []byte

	api2Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/import/json" {
			receivedCallBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"imported":true}`)
			return
		}
		if r.URL.Path == "/storage/upload" {
			// Read multipart
			err := r.ParseMultipartForm(10 << 20)
			if err != nil {
				// Might be direct JSON forward
				body, _ := io.ReadAll(r.Body)
				receivedUploadContent = body
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{"forwarded":true}`)
				return
			}
			file, header, err := r.FormFile("file")
			if err == nil {
				defer file.Close()
				receivedUploadName = header.Filename
				receivedUploadContent, _ = io.ReadAll(file)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"uploaded":true}`)
			return
		}
		if r.URL.Query().Get("nxp") == "json" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"path": "/", "name": "api2"})
			return
		}
		http.NotFound(w, r)
	}))
	defer api2Server.Close()

	// 1. Connect to both APIs
	runConnect([]string{"api1", api1Server.URL})
	runConnect([]string{"api2", api2Server.URL})

	cfg := loadConfig()
	if _, ok := cfg.Endpoints["api1"]; !ok {
		t.Fatalf("api1 not found in registered endpoints")
	}
	if _, ok := cfg.Endpoints["api2"]; !ok {
		t.Fatalf("api2 not found in registered endpoints")
	}

	// 2. Test target resolution
	t1, err := ResolveTarget("@api1/download/report.csv")
	if err != nil || t1.ResolvedURL != api1Server.URL+"/download/report.csv" {
		t.Fatalf("expected resolved target %s, got %s (err: %v)", api1Server.URL+"/download/report.csv", t1.ResolvedURL, err)
	}

	t2, err := ResolveTarget("@api2/storage/upload")
	if err != nil || t2.ResolvedURL != api2Server.URL+"/storage/upload" {
		t.Fatalf("expected resolved target %s, got %s (err: %v)", api2Server.URL+"/storage/upload", t2.ResolvedURL, err)
	}

	// Verify that loose colon syntax is strictly rejected
	if _, err := ResolveTarget("api1:/download/report.csv"); err == nil {
		t.Fatalf("expected strict error for 'api1:/download/report.csv', got nil")
	}

	// 3. Test piping: nxp download @api1/download/report.csv | nxp upload @api2/storage/upload
	pipeReader, pipeWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	// Run download with stdout redirected to pipeWriter
	origStdout := os.Stdout
	os.Stdout = pipeWriter
	runDownload([]string{"@api1/download/report.csv"})
	_ = pipeWriter.Close()

	// Run upload with stdin redirected from pipeReader
	origStdin := os.Stdin
	os.Stdin = pipeReader

	// Capture upload output
	outR, outW, _ := os.Pipe()
	os.Stdout = outW

	runUpload([]string{"@api2/storage/upload", "--filename", "piped_report.csv"})
	_ = outW.Close()
	_, _ = io.ReadAll(outR)
	_ = outR.Close()

	os.Stdout = origStdout
	os.Stdin = origStdin

	if receivedUploadName != "piped_report.csv" {
		t.Errorf("expected upload filename piped_report.csv, got %s", receivedUploadName)
	}
	if !strings.Contains(string(receivedUploadContent), "id,name,value") {
		t.Errorf("expected upload content to contain CSV headers, got %q", string(receivedUploadContent))
	}

	// 4. Test piping API call: nxp call GET @api1/export/json | nxp call POST @api2/import/json
	callR, callW, _ := os.Pipe()
	os.Stdout = callW
	runCall([]string{"GET", "@api1/export/json"})
	_ = callW.Close()

	os.Stdin = callR
	outCallR, outCallW, _ := os.Pipe()
	os.Stdout = outCallW

	runCall([]string{"POST", "@api2/import/json"})
	_ = outCallW.Close()

	os.Stdout = origStdout
	os.Stdin = origStdin

	if !strings.Contains(string(receivedCallBody), `"alpha"`) {
		t.Errorf("expected API 2 to receive JSON containing alpha, got %s", string(receivedCallBody))
	}

	var callOut bytes.Buffer
	_, _ = io.Copy(&callOut, outCallR)
	if !strings.Contains(callOut.String(), `"imported":true`) {
		t.Errorf("expected imported response, got %s", callOut.String())
	}
}

func TestCLIURLForward(t *testing.T) {
	var receivedBody string
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		receivedBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer apiServer.Close()

	runConnect([]string{"uploader", apiServer.URL})

	runUpload([]string{"@uploader/import", "--url", "https://other.com/files/123.pdf"})

	if !strings.Contains(receivedBody, "https://other.com/files/123.pdf") {
		t.Errorf("expected body to contain forwarded URL, got %s", receivedBody)
	}
}
