package config

import (
	"testing"
)

func TestSanitizeServerConfig(t *testing.T) {
	cfg := ServerConfig{
		Host:      "ftp.matrixgames.com/pub/",
		Port:      21,
		Protocol:  "ftp",
		User:      "anonymous",
		Password:  "anything",
		RemoteDir: "/",
	}

	sanitized := SanitizeServerConfig(cfg)
	if sanitized.Host != "ftp.matrixgames.com" {
		t.Errorf("expected host 'ftp.matrixgames.com', got '%s'", sanitized.Host)
	}
	if sanitized.RemoteDir != "/pub" {
		t.Errorf("expected remoteDir '/pub', got '%s'", sanitized.RemoteDir)
	}
	if sanitized.Port != 21 {
		t.Errorf("expected port 21, got %d", sanitized.Port)
	}

	// Test ftp:// URL
	cfg2 := ServerConfig{
		Host: "ftp://files.example.com:2121/downloads/games",
	}
	sanitized2 := SanitizeServerConfig(cfg2)
	if sanitized2.Host != "files.example.com" {
		t.Errorf("expected host 'files.example.com', got '%s'", sanitized2.Host)
	}
	if sanitized2.Port != 2121 {
		t.Errorf("expected port 2121, got %d", sanitized2.Port)
	}
	if sanitized2.RemoteDir != "/downloads/games" {
		t.Errorf("expected remoteDir '/downloads/games', got '%s'", sanitized2.RemoteDir)
	}
	if sanitized2.Protocol != "ftp" {
		t.Errorf("expected protocol 'ftp', got '%s'", sanitized2.Protocol)
	}
	if sanitized2.User != "anonymous" {
		t.Errorf("expected anonymous user, got '%s'", sanitized2.User)
	}
}
