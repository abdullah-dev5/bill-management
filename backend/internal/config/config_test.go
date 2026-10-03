package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "# comment\nBM_TEST_A=one\nexport BM_TEST_B=\"two words\"\nBM_TEST_C='kept'\n\nBM_TEST_SET=from-file\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BM_TEST_SET", "from-env")
	for _, k := range []string{"BM_TEST_A", "BM_TEST_B", "BM_TEST_C"} {
		os.Unsetenv(k)
		t.Cleanup(func() { os.Unsetenv(k) })
	}
	if err := loadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{"BM_TEST_A": "one", "BM_TEST_B": "two words", "BM_TEST_C": "kept", "BM_TEST_SET": "from-env"} {
		if got := os.Getenv(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if err := loadDotEnv(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatalf("missing file: %v", err)
	}
	bad := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(bad, []byte("NOEQUALS\n"), 0o600)
	if err := loadDotEnv(bad); err == nil {
		t.Fatal("expected an error for a malformed line")
	}
}
