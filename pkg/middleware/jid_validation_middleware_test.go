package auth_middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// runJIDValidation runs ValidateJIDFields(fields...) against body and returns the
// response recorder plus the body the downstream handler received (nil when the
// middleware aborted the request).
func runJIDValidation(t *testing.T, body string, fields ...string) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var received map[string]interface{}
	r := gin.New()
	r.POST("/test", NewJIDValidationMiddleware().ValidateJIDFields(fields...), func(c *gin.Context) {
		raw, err := io.ReadAll(c.Request.Body)
		if err != nil {
			t.Fatalf("reading body in handler: %v", err)
		}
		if err := json.Unmarshal(raw, &received); err != nil {
			t.Fatalf("handler got invalid JSON %q: %v", raw, err)
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w, received
}

func TestValidateJIDFields_StringIsNormalized(t *testing.T) {
	w, got := runJIDValidation(t, `{"number":"+55 (11) 3333-4444"}`, "number")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if got["number"] != "+551133334444@s.whatsapp.net" {
		t.Fatalf("number = %v", got["number"])
	}
}

func TestValidateJIDFields_StringAlreadyJIDIsKept(t *testing.T) {
	w, got := runJIDValidation(t, `{"groupJid":"120363000000000000@g.us"}`, "groupJid")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if got["groupJid"] != "120363000000000000@g.us" {
		t.Fatalf("groupJid = %v", got["groupJid"])
	}
}

func TestValidateJIDFields_EmptyStringIsRejected(t *testing.T) {
	w, _ := runJIDValidation(t, `{"number":""}`, "number")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "number is required and cannot be empty") {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestValidateJIDFields_ArrayIsNormalized(t *testing.T) {
	// Same shape ms-wpp sends to POST /group/participant.
	body := `{"groupJid":"120363000000000000@g.us","participants":["5511999998888@s.whatsapp.net","+44 7700 900123"],"action":"add"}`
	w, got := runJIDValidation(t, body, "number", "participants")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	parts, ok := got["participants"].([]interface{})
	if !ok || len(parts) != 2 {
		t.Fatalf("participants = %#v", got["participants"])
	}
	if parts[0] != "5511999998888@s.whatsapp.net" {
		t.Fatalf("participants[0] = %v", parts[0])
	}
	if parts[1] != "+447700900123@s.whatsapp.net" {
		t.Fatalf("participants[1] = %v", parts[1])
	}
	if got["action"] != "add" || got["groupJid"] != "120363000000000000@g.us" {
		t.Fatalf("other fields changed: %#v", got)
	}
}

func TestValidateJIDFields_EmptyArrayIsRejected(t *testing.T) {
	w, _ := runJIDValidation(t, `{"participants":[]}`, "number", "participants")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "participants array cannot be empty") {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestValidateJIDFields_InvalidArrayElementIsRejected(t *testing.T) {
	cases := map[string]string{
		`{"participants":["5511999998888@s.whatsapp.net","abc"]}`: "Invalid participants[1] format",
		`{"participants":["5511999998888@s.whatsapp.net",""]}`:    "participants[1] cannot be empty",
		`{"participants":[123]}`:                                  "participants[0] must be a string",
	}
	for body, want := range cases {
		w, _ := runJIDValidation(t, body, "number", "participants")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", body, w.Code)
		}
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("%s: body = %s, want %q", body, w.Body.String(), want)
		}
	}
}

func TestValidateJIDFields_NonStringNonArrayIsRejected(t *testing.T) {
	w, _ := runJIDValidation(t, `{"number":{"a":1}}`, "number")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "number must be a string or array of strings") {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestValidateJIDFields_MissingFieldPassesThrough(t *testing.T) {
	w, got := runJIDValidation(t, `{"other":"x"}`, "number", "participants")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if got["other"] != "x" {
		t.Fatalf("body changed: %#v", got)
	}
}
