package message_service

import (
	"context"
	"errors"
	"fmt"
	"time"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	"github.com/evolution-foundation/evolution-go/pkg/utils"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// Pin durations supported natively by WhatsApp ("Pin for 24 hours / 7 days / 30 days").
const (
	PinDuration24h uint32 = 86400
	PinDuration7d  uint32 = 604800
	PinDuration30d uint32 = 2592000
)

// PinMessageStruct is the body of POST /message/pin and POST /message/unpin.
type PinMessageStruct struct {
	// Chat JID (user or group) where the message lives.
	Number string `json:"number" example:"120363000000000000@g.us"`
	// ID of the message to pin/unpin.
	Id string `json:"id" example:"3EB0C431C26A1916E3B8"`
	// Pin duration in seconds: 86400 (24h), 604800 (7d) or 2592000 (30d). Defaults to 86400; ignored on unpin.
	Duration uint32 `json:"duration,omitempty" example:"86400"`
	// true to unpin (POST /message/unpin forces it).
	Unpin bool `json:"unpin,omitempty"`
	// Whether the target message was sent by this instance. Defaults to true.
	FromMe *bool `json:"fromMe,omitempty"`
	// Author of the target message in groups when fromMe is false.
	Participant string `json:"participant,omitempty"`
}

// PinMessageResult is returned in "data" by the pin/unpin routes.
type PinMessageResult struct {
	ID        string    `json:"id"`        // ID of the pin/unpin protocol message that was sent
	TargetID  string    `json:"targetId"`  // ID of the pinned/unpinned message
	Chat      string    `json:"chat"`      // chat JID
	Unpin     bool      `json:"unpin"`     // true when it was an unpin
	Duration  uint32    `json:"duration"`  // seconds (0 on unpin)
	Timestamp time.Time `json:"timestamp"` // server timestamp of the send
}

// NormalizePinDuration validates the requested duration. 0 means the default (24h).
func NormalizePinDuration(duration uint32) (uint32, error) {
	switch duration {
	case 0:
		return PinDuration24h, nil
	case PinDuration24h, PinDuration7d, PinDuration30d:
		return duration, nil
	default:
		return 0, fmt.Errorf("invalid duration %d: must be 86400 (24h), 604800 (7d) or 2592000 (30d)", duration)
	}
}

// BuildPinMessage builds the PinInChatMessage protocol message (the same one
// WhatsApp clients send) that pins or unpins the message key for everyone in
// the chat. On pin, the duration is carried in
// MessageContextInfo.MessageAddOnDurationInSecs.
func BuildPinMessage(chat types.JID, targetID string, fromMe bool, participant *types.JID, duration uint32, unpin bool, now time.Time) (*waE2E.Message, error) {
	if targetID == "" {
		return nil, errors.New("id is required")
	}
	key := &waCommon.MessageKey{
		RemoteJID: proto.String(chat.String()),
		FromMe:    proto.Bool(fromMe),
		ID:        proto.String(targetID),
	}
	if participant != nil && !participant.IsEmpty() {
		key.Participant = proto.String(participant.ToNonAD().String())
	}

	pinType := waE2E.PinInChatMessage_PIN_FOR_ALL
	if unpin {
		pinType = waE2E.PinInChatMessage_UNPIN_FOR_ALL
	}
	msg := &waE2E.Message{
		PinInChatMessage: &waE2E.PinInChatMessage{
			Key:               key,
			Type:              pinType.Enum(),
			SenderTimestampMS: proto.Int64(now.UnixMilli()),
		},
	}
	if !unpin {
		d, err := NormalizePinDuration(duration)
		if err != nil {
			return nil, err
		}
		msg.MessageContextInfo = &waE2E.MessageContextInfo{
			MessageAddOnDurationInSecs: proto.Uint32(d),
		}
	}
	return msg, nil
}

func (m *messageService) PinMessage(data *PinMessageStruct, instance *instance_model.Instance) (*PinMessageResult, error) {
	if data.Id == "" {
		return nil, errors.New("missing id in payload")
	}
	duration := uint32(0)
	if !data.Unpin {
		d, err := NormalizePinDuration(data.Duration)
		if err != nil {
			return nil, err
		}
		duration = d
	}

	client, err := m.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	chat, ok := utils.ParseJID(data.Number)
	if !ok {
		m.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return nil, errors.New("invalid phone number")
	}

	fromMe := true
	if data.FromMe != nil {
		fromMe = *data.FromMe
	}

	var participant *types.JID
	if data.Participant != "" {
		p, ok := utils.ParseJID(data.Participant)
		if !ok {
			return nil, errors.New("invalid participant")
		}
		participant = &p
	}

	msg, err := BuildPinMessage(chat, data.Id, fromMe, participant, duration, data.Unpin, time.Now())
	if err != nil {
		return nil, err
	}

	pinID := client.GenerateMessageID()
	resp, err := client.SendMessage(context.Background(), chat, msg, whatsmeow.SendRequestExtra{ID: pinID})
	if err != nil {
		m.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error sending pin message: %v", instance.Id, err)
		return nil, err
	}

	return &PinMessageResult{
		ID:        resp.ID,
		TargetID:  data.Id,
		Chat:      chat.String(),
		Unpin:     data.Unpin,
		Duration:  duration,
		Timestamp: resp.Timestamp,
	}, nil
}
