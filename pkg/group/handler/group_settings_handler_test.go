package group_handler

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	group_service "github.com/evolution-foundation/evolution-go/pkg/group/service"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	"github.com/gin-gonic/gin"
)

// fakeGroupService implements only UpdateGroupSettings; any other method panics
// through the nil embedded interface.
type fakeGroupService struct {
	group_service.GroupService
	got *group_service.UpdateGroupSettingsStruct
	err error
}

func (f *fakeGroupService) UpdateGroupSettings(data *group_service.UpdateGroupSettingsStruct, _ *instance_model.Instance) error {
	f.got = data
	return f.err
}

func postSettings(t *testing.T, svc *fakeGroupService, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewGroupHandler(svc)
	r.POST("/group/settings", func(c *gin.Context) {
		c.Set("instance", &instance_model.Instance{Id: "inst-1"})
		c.Next()
	}, h.UpdateGroupSettings)
	req := httptest.NewRequest(http.MethodPost, "/group/settings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestUpdateGroupSettings_AcceptsMsWppShape(t *testing.T) {
	// Exactly what ms-wpp's evo_go_v2 adapter sends (update_group announce/locked).
	for _, action := range []string{"announcement", "not_announcement", "locked", "unlocked"} {
		svc := &fakeGroupService{}
		w := postSettings(t, svc, `{"groupJid":"120363000000000000@g.us","action":"`+action+`"}`)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, body = %s", action, w.Code, w.Body.String())
		}
		if svc.got == nil || svc.got.GroupJID != "120363000000000000@g.us" || svc.got.Action != action {
			t.Fatalf("%s: service got %#v", action, svc.got)
		}
	}
}

func TestUpdateGroupSettings_ValidationErrors(t *testing.T) {
	cases := map[string]string{
		`{"action":"locked"}`:                                 "groupJid is required",
		`{"groupJid":"120363000000000000@g.us"}`:              "action is required",
		`{"groupJid":"120363000000000000@g.us","action":"x"}`: "invalid action",
		`not json`: "",
	}
	for body, want := range cases {
		svc := &fakeGroupService{}
		w := postSettings(t, svc, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", body, w.Code)
		}
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("%s: body = %s, want %q", body, w.Body.String(), want)
		}
		if svc.got != nil {
			t.Fatalf("%s: service should not be called", body)
		}
	}
}

func TestUpdateGroupSettings_ServiceErrorIs500(t *testing.T) {
	svc := &fakeGroupService{err: errors.New("not-authorized")}
	w := postSettings(t, svc, `{"groupJid":"120363000000000000@g.us","action":"announcement"}`)
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "not-authorized") {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestIsValidGroupSettingsAction(t *testing.T) {
	for _, a := range group_service.GroupSettingsActions {
		if !group_service.IsValidGroupSettingsAction(a) {
			t.Fatalf("%s should be valid", a)
		}
	}
	if group_service.IsValidGroupSettingsAction("ANNOUNCEMENT") || group_service.IsValidGroupSettingsAction("") {
		t.Fatal("unexpected valid action")
	}
}
