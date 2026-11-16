package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	c := Default()
	if c.Version != 1 || c.Theme != ThemeSystem || c.SidebarWidth != 220 {
		t.Fatalf("Default() = %+v", c)
	}
}

func TestNormalize(t *testing.T) {
	c := Config{Theme: "nope", SidebarWidth: 100}
	got := c.Normalize()
	if got.Theme != ThemeSystem {
		t.Fatalf("Theme = %q, want %q", got.Theme, ThemeSystem)
	}
	if got.SidebarWidth != 180 {
		t.Fatalf("SidebarWidth = %d, want 180", got.SidebarWidth)
	}

	c = Config{Theme: ThemeDark, SidebarWidth: 999}
	got = c.Normalize()
	if got.Theme != ThemeDark {
		t.Fatalf("Theme = %q, want %q", got.Theme, ThemeDark)
	}
	if got.SidebarWidth != 360 {
		t.Fatalf("SidebarWidth = %d, want 360", got.SidebarWidth)
	}
}

func TestLoadMissing(t *testing.T) {
	got, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != Default() {
		t.Fatalf("Load = %+v, want Default()", got)
	}
}

func TestLoadCorrupt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err == nil {
		t.Fatal("Load succeeded, want an error")
	}
	if got != Default() {
		t.Fatalf("Load = %+v, want Default()", got)
	}
}

func TestSaveLoad(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "DevToys")
	want := Default()
	want.Theme = ThemeDark
	want.SidebarWidth = 260
	want.LastModule = "welcome"
	if err := want.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Fatalf("Load = %+v, want %+v", got, want)
	}
}
