package whatsmeow_service

import "testing"

func TestInternalRestartMarkIsPerChannelAndConsumedOnce(t *testing.T) {
	restarted := make(chan bool)
	other := make(chan bool)

	if consumeInternalRestart(restarted) {
		t.Fatal("canal sem marca nao deve ser restart interno")
	}

	markInternalRestart(restarted)

	if consumeInternalRestart(other) {
		t.Fatal("marca de um canal nao pode vazar para outro")
	}
	if !consumeInternalRestart(restarted) {
		t.Fatal("kill apos markInternalRestart deve ser restart interno")
	}
	if consumeInternalRestart(restarted) {
		t.Fatal("marca deve ser consumida uma unica vez (kill seguinte = logout real)")
	}
}
