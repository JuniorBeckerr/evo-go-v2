package message_handler

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	message_service "github.com/EvolutionAPI/evolution-go/pkg/message/service"
	"github.com/gin-gonic/gin"
)

// fakeMessageService implements only PinMessage; other methods panic via the nil embedded interface.
type fakeMessageService struct {
	message_service.MessageService
	got *message_service.PinMessageStruct
	err error
}

func (f *fakeMessageService) PinMessage(data *message_service.PinMessageStruct, _ *instance_model.Instance) (*message_service.PinMessageResult, error) {
	f.got = data
	if f.err != nil {
		return nil, f.err
	}
	return &message_service.PinMessageResult{ID: "PINID", TargetID: data.Id, Unpin: data.Unpin}, nil
}

func doPin(t *testing.T, svc *fakeMessageService, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewMessageHandler(svc)
	setInstance := func(c *gin.Context) { c.Set("instance", &instance_model.Instance{Id: "inst-1"}) }
	r.POST("/message/pin", setInstance, h.PinMessage)
	r.POST("/message/unpin", setInstance, h.UnpinMessage)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestPinMessage_OK(t *testing.T) {
	svc := &fakeMessageService{}
	w := doPin(t, svc, "/message/pin", `{"number":"120363000000000000@g.us","id":"MSGID","duration":604800}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"id":"PINID"`) {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if svc.got.Id != "MSGID" || svc.got.Duration != 604800 || svc.got.Unpin {
		t.Fatalf("service got %#v", svc.got)
	}
}

func TestUnpinMessage_ForcesUnpin(t *testing.T) {
	svc := &fakeMessageService{}
	w := doPin(t, svc, "/message/unpin", `{"number":"120363000000000000@g.us","id":"MSGID","duration":5}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !svc.got.Unpin {
		t.Fatal("unpin should be forced on /message/unpin")
	}
}

func TestPinMessage_UnpinFlagOnPinRoute(t *testing.T) {
	svc := &fakeMessageService{}
	w := doPin(t, svc, "/message/pin", `{"number":"120363000000000000@g.us","id":"MSGID","unpin":true}`)
	if w.Code != http.StatusOK || !svc.got.Unpin {
		t.Fatalf("status = %d, got = %#v", w.Code, svc.got)
	}
}

func TestPinMessage_ValidationErrors(t *testing.T) {
	cases := map[string]string{
		`{"id":"MSGID"}`:                       "number is required",
		`{"number":"120363000000000000@g.us"}`: "id is required",
		`{"number":"120363000000000000@g.us","id":"MSGID","duration":60}`: "invalid duration",
	}
	for body, want := range cases {
		svc := &fakeMessageService{}
		w := doPin(t, svc, "/message/pin", body)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), want) {
			t.Fatalf("%s: status = %d, body = %s", body, w.Code, w.Body.String())
		}
		if svc.got != nil {
			t.Fatalf("%s: service should not be called", body)
		}
	}
}

func TestPinMessage_ServiceError(t *testing.T) {
	svc := &fakeMessageService{err: errors.New("client disconnected")}
	w := doPin(t, svc, "/message/pin", `{"number":"120363000000000000@g.us","id":"MSGID"}`)
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "client disconnected") {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}
