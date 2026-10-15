package updater

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"1.0.0", "1.0.0", 0},
		{"v1.0.0", "1.0.0", 0},
		{"v1.2.0", "v1.2.1", -1},
		{"v1.2.1", "v1.2.0", 1},
		{"v1.10.0", "v1.9.0", 1},
		{"v2.0.0", "v1.99.99", 1},
		{"v1.2.0-beta", "v1.2.0", 0},
		{"1.2", "1.2.0", 0},
		{"1", "1.0.0", 0},
		{"v1.2.1", "v1.2.10", -1},
	}

	for _, tc := range tests {
		actual := CompareVersions(tc.v1, tc.v2)
		if actual != tc.expected {
			t.Errorf("CompareVersions(%q, %q) = %d; expected %d", tc.v1, tc.v2, actual, tc.expected)
		}
	}
}

func TestIsNewerVersion(t *testing.T) {
	if !IsNewerVersion("1.2.0", "1.2.1") {
		t.Errorf("expected 1.2.1 to be newer than 1.2.0")
	}
	if IsNewerVersion("1.2.1", "1.2.0") {
		t.Errorf("expected 1.2.0 NOT to be newer than 1.2.1")
	}
	if IsNewerVersion("1.2.0", "1.2.0") {
		t.Errorf("expected 1.2.0 NOT to be newer than 1.2.0")
	}
}
