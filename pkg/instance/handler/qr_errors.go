package instance_handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	instance_service "github.com/evolution-foundation/evolution-go/pkg/instance/service"
)

// qrErrorResponse traduz os estados "QR ainda sendo gerado" (202 pending) e
// "sessao ja autenticada" (409 logged_in) para respostas distintas do 400
// generico, para o cliente que faz polling saber que deve aguardar/parar.
func qrErrorResponse(err error) (int, gin.H, bool) {
	switch {
	case errors.Is(err, instance_service.ErrQrPending):
		return http.StatusAccepted, gin.H{"message": "pending", "status": "pending", "error": err.Error()}, true
	case errors.Is(err, instance_service.ErrAlreadyLoggedIn):
		return http.StatusConflict, gin.H{"message": "logged_in", "status": "logged_in", "error": err.Error()}, true
	}
	return 0, nil, false
}
