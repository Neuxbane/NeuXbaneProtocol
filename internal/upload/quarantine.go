package upload

import (
	"path/filepath"
	"strings"
)

var dangerousExtensions = map[string]bool{
	".exe": true, ".dll": true, ".so": true, ".bin": true,
	".sh": true, ".bat": true, ".cmd": true, ".vbs": true,
	".scr": true, ".com": true,
}

// ShouldQuarantine inspects filename and mime type to determine if a file should be quarantined.
func ShouldQuarantine(filename, mime string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	if dangerousExtensions[ext] {
		return true
	}
	if strings.Contains(mime, "executable") || strings.Contains(mime, "x-dosexec") {
		return true
	}
	return false
}
