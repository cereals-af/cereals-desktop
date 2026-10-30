package appupdate

import "testing"

func TestSupportedInstallationMatchesPayloadShape(t *testing.T) {
	for _, tc := range []struct {
		platform, executable string
		want                 bool
	}{
		{"darwin", "/Applications/cereals.app/Contents/MacOS/cereals", true},
		{"darwin", "/workspace/bin/cereals", false},
		{"darwin", "/workspace/cereals.app/cereals", false},
		{"darwin", "/workspace/cereals/Contents/MacOS/cereals", false},
		{"windows", `C:\Users\test\Programs\cereals\cereals.exe`, true},
		{"linux", "/usr/bin/cereals", false},
	} {
		if got := SupportsInstallation(tc.platform, tc.executable); got != tc.want {
			t.Errorf("%s %s supported=%v, want %v", tc.platform, tc.executable, got, tc.want)
		}
	}
}
