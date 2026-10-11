package whatsmeow_service

import (
	"testing"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
)

func newPresenceTestClient(t *testing.T, kill chan bool) *MyClient {
	t.Helper()
	return &MyClient{
		userID:        "inst-1",
		killChannel:   map[string](chan bool){"inst-1": kill},
		loggerWrapper: logger_wrapper.NewLoggerManager(&config.Config{LogDirectory: t.TempDir()}),
		done:          make(chan struct{}),
	}
}

// A goroutine de presenca nao pode consumir o kill channel: o sinal e de
// StartClient (unico consumidor), senao o restart interno/logout e perdido.
func TestPresenceGoroutineDoesNotConsumeKillSignal(t *testing.T) {
	kill := make(chan bool)
	mycli := newPresenceTestClient(t, kill)

	exited := make(chan struct{})
	go func() { schedulePresenceUpdates(mycli); close(exited) }()

	// Unico receptor possivel e a goroutine de presenca: o envio deve ficar
	// pendente (ninguem "rouba" o sinal destinado a StartClient).
	select {
	case kill <- true:
		t.Fatal("goroutine de presenca consumiu o sinal de kill destinado a StartClient")
	case <-time.After(200 * time.Millisecond):
	}

	// Fim do cliente (close(done)) encerra a goroutine de presenca.
	close(mycli.done)
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine de presenca nao terminou apos done ser fechado (leak)")
	}
}

// Varios Connected nao podem empilhar goroutines de presenca.
func TestPresenceGoroutineSingleInstancePerClient(t *testing.T) {
	mycli := newPresenceTestClient(t, make(chan bool))

	first := make(chan struct{})
	go func() { schedulePresenceUpdates(mycli); close(first) }()
	deadline := time.Now().Add(2 * time.Second)
	for !mycli.presenceRunning.Load() {
		if time.Now().After(deadline) {
			t.Fatal("primeira goroutine de presenca nao iniciou")
		}
		time.Sleep(time.Millisecond)
	}

	second := make(chan struct{})
	go func() { schedulePresenceUpdates(mycli); close(second) }()
	select {
	case <-second:
	case <-time.After(2 * time.Second):
		t.Fatal("segunda goroutine de presenca deveria retornar imediatamente")
	}

	close(mycli.done)
	select {
	case <-first:
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine de presenca nao terminou apos done")
	}
}
