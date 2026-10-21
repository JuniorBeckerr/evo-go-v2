package instance_service

import (
	"testing"
	"time"
)

func TestStartGuardAllowsOneStartPerWindow(t *testing.T) {
	now := time.Unix(1000, 0)
	g := newStartGuard(30*time.Second, func() time.Time { return now })

	if !g.begin("a") {
		t.Fatal("primeira chamada deve iniciar o cliente")
	}
	if g.begin("a") {
		t.Fatal("poll seguinte nao pode reiniciar o cliente que ainda esta subindo")
	}
	if !g.begin("b") {
		t.Fatal("outra instancia tem a sua propria janela")
	}
	if !g.starting("a") {
		t.Fatal("a deve constar como subindo")
	}

	now = now.Add(31 * time.Second)
	if g.starting("a") {
		t.Fatal("janela expirada nao conta mais como subindo")
	}
	if !g.begin("a") {
		t.Fatal("apos a janela (start falhou), nova tentativa e permitida")
	}

	g.clear("a")
	if g.starting("a") || !g.begin("a") {
		t.Fatal("clear libera a instancia")
	}
}

func TestQrStateDecision(t *testing.T) {
	cases := []struct {
		name                               string
		hasClient, loggedIn, connected     bool
		want                               qrAction
	}{
		{"sem cliente", false, false, false, qrStartClient},
		{"conectado e logado", true, true, true, qrAlreadyLoggedIn},
		{"logado mas socket caido", true, true, false, qrStartClient},
		{"cliente subindo/aguardando QR", true, false, false, qrReadStored},
		{"socket aberto aguardando scan", true, false, true, qrReadStored},
	}
	for _, c := range cases {
		if got := decideQrAction(c.hasClient, c.loggedIn, c.connected); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
