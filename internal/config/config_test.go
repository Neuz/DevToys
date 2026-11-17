package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	c := Default()
	if c.Theme != ThemeSystem || c.Locale != LocaleSystem {
		t.Fatalf("Default() = %+v", c)
	}
}

func TestNormalize(t *testing.T) {
	c := Config{Theme: "nope", Locale: "fr"}
	got := c.Normalize()
	if got.Theme != ThemeSystem {
		t.Fatalf("Theme = %q, want %q", got.Theme, ThemeSystem)
	}
	if got.Locale != LocaleSystem {
		t.Fatalf("Locale = %q, want %q", got.Locale, LocaleSystem)
	}

	c = Config{Theme: ThemeDark, Locale: LocaleChinese}
	got = c.Normalize()
	if got.Theme != ThemeDark {
		t.Fatalf("Theme = %q, want %q", got.Theme, ThemeDark)
	}
	if got.Locale != LocaleChinese {
		t.Fatalf("Locale = %q, want %q", got.Locale, LocaleChinese)
	}
}

func TestLoadMissing(t *testing.T) {
	got, err := LoadFrom(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != Default() {
		t.Fatalf("Load = %+v, want Default()", got)
	}
}

func TestLoadCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, fileName)
	const broken = "{nope"
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFrom(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != Default() {
		t.Fatalf("Load = %+v, want Default()", got)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("损坏的 config.json 未被移走: %v", err)
	}
	backup, err := os.ReadFile(path + ".bad")
	if err != nil {
		t.Fatalf("读取备份: %v", err)
	}
	if string(backup) != broken {
		t.Fatalf("备份内容 = %q, want %q", backup, broken)
	}
}

func TestSaveLoad(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "DevToys")
	want := Default()
	want.Theme = ThemeDark
	want.Locale = LocaleChinese
	if err := want.SaveTo(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := LoadFrom(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Fatalf("Load = %+v, want %+v", got, want)
	}
}
