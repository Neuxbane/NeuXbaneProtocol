// Package download answers: how are files streamed to clients with Range, ETag, and RFC 5987 Content-Disposition?
package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Neuxbane/NeuXbaneProtocol/internal/storage"
)

// ServeFile streams a file from storage to the http.ResponseWriter with Range, ETag, and RFC 5987 Content-Disposition.
func ServeFile(ctx context.Context, w http.ResponseWriter, r *http.Request, backend storage.Backend, fileID, filename string) error {
	rc, size, err := backend.Get(ctx, fileID)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return err
	}
	defer rc.Close()

	// ETag
	etag := fmt.Sprintf(`W/"%s-%d"`, fileID, size)
	w.Header().Set("ETag", etag)
	w.Header().Set("Accept-Ranges", "bytes")

	// RFC 5987 Content-Disposition
	encodedFilename := url.PathEscape(filename)
	disposition := fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, filename, encodedFilename)
	w.Header().Set("Content-Disposition", disposition)

	// Conditional GET
	if ifNoneMatch := r.Header.Get("If-None-Match"); ifNoneMatch != "" {
		if ifNoneMatch == etag || ifNoneMatch == "*" {
			w.WriteHeader(http.StatusNotModified)
			return nil
		}
	}

	// Range request handling
	rangeHeader := r.Header.Get("Range")
	if strings.HasPrefix(rangeHeader, "bytes=") {
		parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
		start, _ := strconv.ParseInt(parts[0], 10, 64)
		end := size - 1
		if len(parts) > 1 && parts[1] != "" {
			if endVal, err := strconv.ParseInt(parts[1], 10, 64); err == nil && endVal < size {
				end = endVal
			}
		}

		if start >= 0 && start <= end && end < size {
			length := end - start + 1
			rangeRC, err := backend.GetRange(ctx, fileID, start, length)
			if err == nil {
				defer rangeRC.Close()
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
				w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
				w.WriteHeader(http.StatusPartialContent)

				if r.Method == http.MethodHead {
					return nil
				}

				_, err = io.Copy(w, rangeRC)
				return err
			}
		}
	}

	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return nil
	}

	_, err = io.Copy(w, rc)
	return err
}
