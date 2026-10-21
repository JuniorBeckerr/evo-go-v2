package instance_service

import (
	"errors"
	"sync"
	"time"
)

// ErrQrPending: o cliente ja esta subindo (ou o socket esta aberto) mas o QR
// ainda nao foi emitido pelo WhatsApp. O handler responde 202 "pending" para
// quem faz polling tentar de novo, em vez de 400.
var ErrQrPending = errors.New("qr code pending")

// ErrAlreadyLoggedIn: a sessao ja esta autenticada e conectada (nao ha QR).
var ErrAlreadyLoggedIn = errors.New("session already logged in")

// startGuard evita que cada GET /instance/qr (polling) reinicie o cliente:
// so um StartInstance por instancia dentro da janela. Se o start falhar, a
// janela expira e a proxima chamada tenta de novo.
type startGuard struct {
	mu     sync.Mutex
	window time.Duration
	now    func() time.Time
	at     map[string]time.Time
}

func newStartGuard(window time.Duration, now func() time.Time) *startGuard {
	if now == nil {
		now = time.Now
	}
	return &startGuard{window: window, now: now, at: map[string]time.Time{}}
}

// begin reserva o start; false se outro start da mesma instancia esta em curso.
func (g *startGuard) begin(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if t, ok := g.at[id]; ok && g.now().Sub(t) < g.window {
		return false
	}
	g.at[id] = g.now()
	return true
}

func (g *startGuard) starting(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	t, ok := g.at[id]
	return ok && g.now().Sub(t) < g.window
}

func (g *startGuard) clear(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.at, id)
}

// clientStarts e compartilhado por GetQr e Pair.
var clientStarts = newStartGuard(30*time.Second, nil)

type qrAction int

const (
	qrReadStored qrAction = iota
	qrStartClient
	qrAlreadyLoggedIn
)

func (a qrAction) String() string {
	switch a {
	case qrStartClient:
		return "start"
	case qrAlreadyLoggedIn:
		return "logged_in"
	default:
		return "read"
	}
}

// decideQrAction: sem cliente (ou logado com socket caido) precisa subir um
// cliente; logado e conectado nao tem QR; qualquer outro estado e um cliente
// em pareamento cujo QR deve ser lido do banco (sem reinicia-lo).
func decideQrAction(hasClient, loggedIn, connected bool) qrAction {
	if !hasClient {
		return qrStartClient
	}
	if loggedIn {
		if connected {
			return qrAlreadyLoggedIn
		}
		return qrStartClient
	}
	return qrReadStored
}
