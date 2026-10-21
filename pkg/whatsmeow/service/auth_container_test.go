package whatsmeow_service

import (
	"fmt"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSharedAuthContainerIsReusedAcrossStarts(t *testing.T) {
	resetSharedAuthContainers()
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_busy_timeout=5000", filepath.Join(t.TempDir(), "auth.db"))

	first, err := sharedAuthContainer("sqlite", dsn, nil)
	if err != nil {
		t.Fatalf("primeira criacao falhou: %v", err)
	}
	second, err := sharedAuthContainer("sqlite", dsn, nil)
	if err != nil {
		t.Fatalf("segunda chamada falhou: %v", err)
	}
	if first != second {
		t.Fatal("cada StartClient abria um pool novo (vazava conexoes ate 'too many clients'); o container deve ser compartilhado")
	}
}

func TestSharedAuthContainerDoesNotCacheFailures(t *testing.T) {
	resetSharedAuthContainers()
	if _, err := sharedAuthContainer("dialeto-invalido", "x", nil); err == nil {
		t.Fatal("dialeto invalido deveria falhar")
	}
	if _, ok := sharedAuthContainers.m["dialeto-invalido|x"]; ok {
		t.Fatal("falha nao pode ficar em cache (proxima tentativa deve reabrir)")
	}
}
