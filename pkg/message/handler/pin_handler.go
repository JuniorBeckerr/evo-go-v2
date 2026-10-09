package message_handler

import (
	"net/http"

	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	message_service "github.com/EvolutionAPI/evolution-go/pkg/message/service"
	"github.com/gin-gonic/gin"
)

// Pin a message
// @Summary Pin a message in a chat
// @Description Pins (or, with "unpin": true, unpins) a message for everyone in the chat by sending a PinInChatMessage. duration is in seconds: 86400 (24h, default), 604800 (7d) or 2592000 (30d). fromMe defaults to true; for someone else's message in a group send fromMe=false and participant. In groups where only admins can pin, the instance must be admin.
// @Tags Message
// @Accept json
// @Produce json
// @Param message body message_service.PinMessageStruct true "Pin message data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /message/pin [post]
func (m *messageHandler) PinMessage(ctx *gin.Context) {
	m.pinMessage(ctx, false)
}

// Unpin a message
// @Summary Unpin a message in a chat
// @Description Unpins a message for everyone in the chat (same body as /message/pin; "unpin" is forced to true and duration is ignored).
// @Tags Message
// @Accept json
// @Produce json
// @Param message body message_service.PinMessageStruct true "Unpin message data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /message/unpin [post]
func (m *messageHandler) UnpinMessage(ctx *gin.Context) {
	m.pinMessage(ctx, true)
}

func (m *messageHandler) pinMessage(ctx *gin.Context, forceUnpin bool) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *message_service.PinMessageStruct
	if err := ctx.ShouldBindBodyWithJSON(&data); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if data == nil || data.Number == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "number is required"})
		return
	}
	if data.Id == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	if forceUnpin {
		data.Unpin = true
	}
	if !data.Unpin {
		if _, err := message_service.NormalizePinDuration(data.Duration); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	result, err := m.messageService.PinMessage(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": result})
}
