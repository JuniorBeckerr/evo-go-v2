package message_service

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

func TestNormalizePinDuration(t *testing.T) {
	cases := map[uint32]uint32{0: 86400, 86400: 86400, 604800: 604800, 2592000: 2592000}
	for in, want := range cases {
		got, err := NormalizePinDuration(in)
		if err != nil || got != want {
			t.Fatalf("NormalizePinDuration(%d) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []uint32{1, 3600, 86401, 7776000} {
		if _, err := NormalizePinDuration(bad); err == nil {
			t.Fatalf("NormalizePinDuration(%d) should fail", bad)
		}
	}
}

func TestBuildPinMessage_Pin(t *testing.T) {
	chat := types.NewJID("120363000000000000", types.GroupServer)
	now := time.UnixMilli(1700000000123)
	msg, err := BuildPinMessage(chat, "MSGID", true, nil, PinDuration7d, false, now)
	if err != nil {
		t.Fatal(err)
	}
	pin := msg.GetPinInChatMessage()
	if pin == nil {
		t.Fatal("PinInChatMessage missing")
	}
	if pin.GetType() != waE2E.PinInChatMessage_PIN_FOR_ALL {
		t.Fatalf("type = %v", pin.GetType())
	}
	if pin.GetSenderTimestampMS() != 1700000000123 {
		t.Fatalf("ts = %d", pin.GetSenderTimestampMS())
	}
	key := pin.GetKey()
	if key.GetRemoteJID() != "120363000000000000@g.us" || key.GetID() != "MSGID" || !key.GetFromMe() || key.Participant != nil {
		t.Fatalf("key = %v", key)
	}
	if msg.GetMessageContextInfo().GetMessageAddOnDurationInSecs() != PinDuration7d {
		t.Fatalf("duration = %d", msg.GetMessageContextInfo().GetMessageAddOnDurationInSecs())
	}
}

func TestBuildPinMessage_DefaultDurationAndParticipant(t *testing.T) {
	chat := types.NewJID("120363000000000000", types.GroupServer)
	participant := types.JID{User: "5511999998888", Device: 3, Server: types.DefaultUserServer}
	msg, err := BuildPinMessage(chat, "MSGID", false, &participant, 0, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	key := msg.GetPinInChatMessage().GetKey()
	if key.GetFromMe() || key.GetParticipant() != "5511999998888@s.whatsapp.net" {
		t.Fatalf("key = %v", key)
	}
	if msg.GetMessageContextInfo().GetMessageAddOnDurationInSecs() != PinDuration24h {
		t.Fatalf("duration = %d", msg.GetMessageContextInfo().GetMessageAddOnDurationInSecs())
	}
}

func TestBuildPinMessage_Unpin(t *testing.T) {
	chat := types.NewJID("5511999998888", types.DefaultUserServer)
	// duration is ignored (even if invalid) on unpin
	msg, err := BuildPinMessage(chat, "MSGID", true, nil, 123, true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if msg.GetPinInChatMessage().GetType() != waE2E.PinInChatMessage_UNPIN_FOR_ALL {
		t.Fatalf("type = %v", msg.GetPinInChatMessage().GetType())
	}
	if msg.MessageContextInfo != nil {
		t.Fatalf("unpin should not carry duration: %v", msg.MessageContextInfo)
	}
}

func TestBuildPinMessage_Errors(t *testing.T) {
	chat := types.NewJID("5511999998888", types.DefaultUserServer)
	if _, err := BuildPinMessage(chat, "", true, nil, 0, false, time.Now()); err == nil {
		t.Fatal("empty id should fail")
	}
	if _, err := BuildPinMessage(chat, "MSGID", true, nil, 42, false, time.Now()); err == nil {
		t.Fatal("invalid duration should fail")
	}
}
