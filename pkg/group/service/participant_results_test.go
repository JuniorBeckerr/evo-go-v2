package group_service

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow/types"
)

func TestParticipantResults_MapsPerParticipantOutcome(t *testing.T) {
	exp := time.Unix(1791700000, 0)
	in := []types.GroupParticipant{
		{JID: types.NewJID("554599242689", types.DefaultUserServer)},
		{JID: types.NewJID("5511900000001", types.DefaultUserServer), Error: 403,
			AddRequest: &types.GroupParticipantAddRequest{Code: "ABCD", Expiration: exp}},
		{JID: types.NewJID("123", types.HiddenUserServer), PhoneNumber: types.NewJID("5511900000002", types.DefaultUserServer), Error: 409},
	}
	got := ParticipantResults(in)
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].JID != "554599242689@s.whatsapp.net" || got[0].Status != 200 || got[0].Error != 0 || got[0].AddRequest != nil {
		t.Fatalf("ok participant = %#v", got[0])
	}
	if got[1].Status != 403 || got[1].Error != 403 || got[1].AddRequest == nil ||
		got[1].AddRequest.Code != "ABCD" || got[1].AddRequest.Expiration != exp.Unix() {
		t.Fatalf("privacy participant = %#v", got[1])
	}
	if got[2].Status != 409 || got[2].PhoneNumber != "5511900000002@s.whatsapp.net" || got[2].JID != "123@lid" {
		t.Fatalf("already-member participant = %#v", got[2])
	}
}

func TestParticipantResults_Empty(t *testing.T) {
	if got := ParticipantResults(nil); got == nil || len(got) != 0 {
		t.Fatalf("got %#v, want empty non-nil slice", got)
	}
}
