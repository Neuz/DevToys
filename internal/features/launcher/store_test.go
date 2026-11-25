package launcher

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestStoreIndexRoundTrip(t *testing.T) {
	st, err := openStoreAt(filepath.Join(t.TempDir(), "launcher.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	entries := []Entry{{
		Title:       "计算器",
		ID:          "Microsoft.WindowsCalculator_8wekyb3d8bbwe!App",
		Path:        `shell:AppsFolder\Microsoft.WindowsCalculator_8wekyb3d8bbwe!App`,
		Keywords:    "计算器 Microsoft.WindowsCalculator_8wekyb3d8bbwe jisuanqi jsq",
		Description: "",
	}}
	if err := st.SaveIndex(entries); err != nil {
		t.Fatal(err)
	}

	if got := st.Entries(); !reflect.DeepEqual(got, entries) {
		t.Errorf("entries = %+v, want %+v", got, entries)
	}
}

func TestStoreReplacesPreviousIndex(t *testing.T) {
	st, err := openStoreAt(filepath.Join(t.TempDir(), "launcher.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.SaveIndex([]Entry{{Title: "Old", Path: "old"}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveIndex([]Entry{{Title: "New", Path: "new"}}); err != nil {
		t.Fatal(err)
	}
	got := st.Entries()
	if len(got) != 1 || got[0].Path != "new" {
		t.Fatalf("entries = %+v, want only new", got)
	}
}

func TestStoreUsageRoundTrip(t *testing.T) {
	st, err := openStoreAt(filepath.Join(t.TempDir(), "launcher.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	usage := map[string]Usage{
		"a": {Count: 2, Last: time.Unix(1700000000, 0)},
		"b": {Count: 1},
	}
	if err := st.SaveUsage(usage); err != nil {
		t.Fatal(err)
	}
	got := st.Usage()
	if len(got) != len(usage) {
		t.Fatalf("usage = %+v, want %d entries", got, len(usage))
	}
	for k, want := range usage {
		g, ok := got[k]
		// JSON 往返后 time.Time 的 Location 指针会变（Local ↔ UTC），
		// DeepEqual 会误报；按时刻语义比较。
		if !ok || g.Count != want.Count || !g.Last.Equal(want.Last) {
			t.Errorf("usage[%s] = %+v, want %+v", k, g, want)
		}
	}
}

func TestStoreEmpty(t *testing.T) {
	st, err := openStoreAt(filepath.Join(t.TempDir(), "launcher.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if entries := st.Entries(); len(entries) != 0 {
		t.Errorf("entries = %+v, want empty", entries)
	}
	if got := st.Usage(); got != nil {
		t.Errorf("usage = %+v, want nil", got)
	}
}

func TestNilStoreIsANoop(t *testing.T) {
	var st *store
	if err := st.SaveIndex([]Entry{{Path: "x"}}); err != nil {
		t.Errorf("SaveIndex = %v", err)
	}
	if err := st.SaveUsage(map[string]Usage{"x": {}}); err != nil {
		t.Errorf("SaveUsage = %v", err)
	}
	if entries := st.Entries(); entries != nil {
		t.Errorf("Entries = %+v, want nil", entries)
	}
	if usage := st.Usage(); usage != nil {
		t.Errorf("Usage = %+v, want nil", usage)
	}
	if err := st.Close(); err != nil {
		t.Errorf("Close = %v", err)
	}
}
