package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPathFollowsThePlatformConfigDir(t *testing.T) {
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join(dir, "lsm", "config.yaml")
	if got != want {
		t.Fatalf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestResolvePathPrecedence(t *testing.T) {
	def, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}

	t.Run("platform default", func(t *testing.T) {
		t.Setenv("LSM_CONFIG", "")
		got, err := ResolvePath("")
		if err != nil {
			t.Fatal(err)
		}
		if got != def {
			t.Fatalf("got %q, want %q", got, def)
		}
	})

	t.Run("LSM_CONFIG overrides the default", func(t *testing.T) {
		t.Setenv("LSM_CONFIG", "/env/lsm.yaml")
		got, err := ResolvePath("")
		if err != nil {
			t.Fatal(err)
		}
		if got != "/env/lsm.yaml" {
			t.Fatalf("got %q, want %q", got, "/env/lsm.yaml")
		}
	})

	t.Run("--config wins over LSM_CONFIG", func(t *testing.T) {
		t.Setenv("LSM_CONFIG", "/env/lsm.yaml")
		got, err := ResolvePath("/flag/lsm.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if got != "/flag/lsm.yaml" {
			t.Fatalf("got %q, want %q", got, "/flag/lsm.yaml")
		}
	})
}

func TestRunAndPIDPathsSitNextToTheConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "nested", "config.yaml")

	if got, want := RunDir(cfgPath), filepath.Join(filepath.Dir(cfgPath), "run"); got != want {
		t.Fatalf("RunDir() = %q, want %q", got, want)
	}
	want := filepath.Join(filepath.Dir(cfgPath), "run", "web.pid")
	if got := PIDPath(cfgPath, "web"); got != want {
		t.Fatalf("PIDPath() = %q, want %q", got, want)
	}
}
