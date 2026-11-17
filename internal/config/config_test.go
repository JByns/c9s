package config

import (
	"os"
	"path/filepath"
	"testing"
)

func setHome(t *testing.T) string {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("C9S_CONTROLLER", "")
	return home
}

func TestDetectFromClientConfig(t *testing.T) {
	home := setHome(t)
	os.MkdirAll(filepath.Join(home, ".s9s"), 0o700)
	os.WriteFile(filepath.Join(home, ".s9s", "s9s.conf"), []byte("[global]\ncmon_user=dba\ncontroller=https://localhost:9501\n# controller=https://other:9501\n"), 0o600)
	os.WriteFile(filepath.Join(home, ".s9s", "dba.key"), []byte("x"), 0o600)

	l, err := Detect("", "")
	if err != nil {
		t.Fatal(err)
	}
	if l.Controller != "https://localhost:9501" || l.User != "dba" || l.Key != filepath.Join(home, ".s9s", "dba.key") {
		t.Fatalf("unexpected %+v", l)
	}
	if l, _ = Detect("admin", ""); l.User != "admin" || l.Key != "" {
		t.Fatalf("user override: %+v", l)
	}
}

func TestDetectDefaultsToLocalCmon(t *testing.T) {
	setHome(t)
	l, err := Detect("", "")
	if err != nil {
		t.Fatal(err)
	}
	if l.Controller != DefaultController || l.User != "" || l.Key != "" {
		t.Fatalf("unexpected %+v", l)
	}
}

func TestDetectRefusesRemoteController(t *testing.T) {
	setHome(t)
	t.Setenv("C9S_CONTROLLER", "https://192.0.2.10:9501")
	if _, err := Detect("", ""); err == nil {
		t.Fatal("a controller on another machine must be refused")
	}
}

func TestIsLocal(t *testing.T) {
	for u, want := range map[string]bool{
		"https://127.0.0.1:9501":  true,
		"https://localhost:9501":  true,
		"http://[::1]:9501":       true,
		"https://192.0.2.10:9501": false,
	} {
		if got := IsLocal(u); got != want {
			t.Errorf("IsLocal(%s) = %v", u, got)
		}
	}
	hn, _ := os.Hostname()
	if !IsLocal("https://" + hn + ":9501") {
		t.Error("own host name must be local")
	}
}
