package whatsmeow_service

import (
	"context"
	"sync"

	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// sharedAuthContainers guarda um unico sqlstore.Container por (dialeto, DSN).
//
// Antes, cada StartClient (inclusive os restarts internos disparados a cada
// Disconnected do WhatsApp) chamava sqlstore.New, que abre um *sql.DB novo e
// nunca o fecha. Com o socket caindo algumas vezes por dia, as conexoes ociosas
// se acumulavam no Postgres ate "too many clients already" — e dai nenhum
// cliente conseguia mais subir (nem para reconectar, nem para gerar QR).
var sharedAuthContainers = struct {
	sync.Mutex
	m map[string]*sqlstore.Container
}{m: map[string]*sqlstore.Container{}}

func sharedAuthContainer(dialect, dsn string, log waLog.Logger) (*sqlstore.Container, error) {
	key := dialect + "|" + dsn
	sharedAuthContainers.Lock()
	defer sharedAuthContainers.Unlock()
	if c, ok := sharedAuthContainers.m[key]; ok {
		return c, nil
	}
	c, err := sqlstore.New(context.Background(), dialect, dsn, log)
	if err != nil {
		// Nao guarda falhas: a proxima tentativa reabre do zero.
		return nil, err
	}
	sharedAuthContainers.m[key] = c
	return c, nil
}

// resetSharedAuthContainers e usado apenas nos testes.
func resetSharedAuthContainers() {
	sharedAuthContainers.Lock()
	defer sharedAuthContainers.Unlock()
	sharedAuthContainers.m = map[string]*sqlstore.Container{}
}
