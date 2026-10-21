package instance_handler

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	instance_service "github.com/evolution-foundation/evolution-go/pkg/instance/service"
)

func TestQrErrorResponse(t *testing.T) {
	status, body, ok := qrErrorResponse(fmt.Errorf("wrap: %w", instance_service.ErrQrPending))
	if !ok || status != http.StatusAccepted || body["status"] != "pending" {
		t.Fatalf("QR em geracao deve ser 202 pending, got %v %v %v", ok, status, body)
	}
	status, body, ok = qrErrorResponse(instance_service.ErrAlreadyLoggedIn)
	if !ok || status != http.StatusConflict || body["status"] != "logged_in" {
		t.Fatalf("sessao ja logada deve ser 409 logged_in, got %v %v %v", ok, status, body)
	}
	if _, _, ok := qrErrorResponse(errors.New("outro")); ok {
		t.Fatal("erros genericos seguem o caminho antigo")
	}
}
