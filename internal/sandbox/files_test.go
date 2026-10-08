package sandbox

import "testing"

func TestWorkspaceDownloadPaths(t *testing.T) {
	for _, value := range []string{"../secret", "/etc/passwd", ".browser/Default/Cookies", "output/../.browser/Cookies", "output/.secret", "", "file\x00.png"} {
		if _, err := WorkspacePath(value); err == nil {
			t.Errorf("accepted private path %q", value)
		}
	}
	for _, value := range []string{"image.png", "/workspace/output/my image.png"} {
		if _, err := WorkspacePath(value); err != nil {
			t.Errorf("rejected file %q: %v", value, err)
		}
	}
}
