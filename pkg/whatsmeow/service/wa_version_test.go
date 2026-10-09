package whatsmeow_service

import (
	"testing"

	"go.mau.fi/whatsmeow/store"
)

func TestEnsureMinWAVersion(t *testing.T) {
	orig := store.GetWAVersion()
	defer store.SetWAVersion(orig)

	store.SetWAVersion(store.WAVersionContainer{2, 3000, 1})
	EnsureMinWAVersion()
	if store.GetWAVersion() != MinWAVersion {
		t.Fatalf("expected %v, got %v", MinWAVersion, store.GetWAVersion())
	}

	newer := store.WAVersionContainer{2, 3000, MinWAVersion[2] + 1}
	store.SetWAVersion(newer)
	EnsureMinWAVersion()
	if store.GetWAVersion() != newer {
		t.Fatalf("must not downgrade: got %v", store.GetWAVersion())
	}
}

func TestMinWAVersionAppliedAtInit(t *testing.T) {
	if store.GetWAVersion().LessThan(MinWAVersion) {
		t.Fatalf("init did not apply MinWAVersion: %v", store.GetWAVersion())
	}
}
