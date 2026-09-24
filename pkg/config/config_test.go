package config

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSanitize_DownloadPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Testing Windows drive root normalization")
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"F:", `F:\Ducke`},
		{`F:\`, `F:\Ducke`},
		{"D:", `D:\Ducke`},
		{`D:\`, `D:\Ducke`},
		{`C:\`, `C:\Ducke`},
		{`F:\Games`, `F:\Games`},
		{`D:\MyFolder\Games`, `D:\MyFolder\Games`},
		{"", `C:\Ducke`},
	}

	for _, tc := range tests {
		s := &AppSettings{
			DownloadPath: tc.input,
		}
		s.Sanitize()
		cleanGot := filepath.Clean(s.DownloadPath)
		cleanExp := filepath.Clean(tc.expected)
		if !strings.EqualFold(cleanGot, cleanExp) {
			t.Errorf("Sanitize(%q) = %q, expected %q", tc.input, s.DownloadPath, tc.expected)
		}
	}
}
