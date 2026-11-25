package translate

import (
	"encoding/json"
	"testing"

	"github.com/Neuz/DevToys/internal/config"
)

func TestDefaultOptions(t *testing.T) {
	opts := defaultOptions()
	if opts.Target != langAuto {
		t.Fatalf("Target = %q, want auto", opts.Target)
	}
	if !opts.PanelFollowsCursor {
		t.Fatal("PanelFollowsCursor should default to true")
	}
	if !opts.Providers.Bilibili || opts.Providers.Google {
		t.Fatalf("providers = %+v, want only bilibili", opts.Providers)
	}
}

func TestDecodeOptionsKeepsDefaults(t *testing.T) {
	node := &config.Feature{Options: json.RawMessage(`{"providers":{"google":true}}`)}
	opts := decodeOptions(node)
	if !opts.Providers.Google || !opts.Providers.Bilibili {
		t.Fatalf("providers = %+v, want google and bilibili", opts.Providers)
	}
	if !opts.PanelFollowsCursor || opts.Target != langAuto {
		t.Fatalf("defaults lost: %+v", opts)
	}
}

func TestDecodeOptionsInvalidTarget(t *testing.T) {
	node := &config.Feature{Options: json.RawMessage(`{"target":"klingon"}`)}
	if opts := decodeOptions(node); opts.Target != langAuto {
		t.Fatalf("Target = %q, want auto", opts.Target)
	}
}

func TestDecodeOptionsMissingNode(t *testing.T) {
	if opts := decodeOptions(nil); !opts.Providers.Bilibili {
		t.Fatalf("providers = %+v, want bilibili default", opts.Providers)
	}
}

func TestNewServiceProviderOrder(t *testing.T) {
	svc := NewService(nil, options{Providers: providerOptions{Google: true, Bilibili: true}})
	ids := svc.IDs()
	want := []string{providerGoogle, providerBilibili}
	if len(ids) != len(want) {
		t.Fatalf("IDs() = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("IDs() = %v, want %v", ids, want)
		}
	}
}

func TestNewServiceNoProviders(t *testing.T) {
	if ids := NewService(nil, options{}).IDs(); len(ids) != 0 {
		t.Fatalf("IDs() = %v, want empty", ids)
	}
}
