package webhook_producer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestSignPayload_KnownVector(t *testing.T) {
	// RFC 4231 test case 2: key "Jefe", data "what do ya want for nothing?"
	got := SignPayload("Jefe", []byte("what do ya want for nothing?"))
	want := "sha256=5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843"
	if got != want {
		t.Fatalf("SignPayload = %s, want %s", got, want)
	}
}

func TestSignPayload_MatchesStdlibHMAC(t *testing.T) {
	body := []byte(`{"event":"Message","data":{"text":"olá 🌎"},"instanceId":"x"}`)
	mac := hmac.New(sha256.New, []byte("s3cr3t"))
	mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if got := SignPayload("s3cr3t", body); got != want {
		t.Fatalf("SignPayload = %s, want %s", got, want)
	}
}

func TestVerifyPayload(t *testing.T) {
	body := []byte(`{"a":1}`)
	sig := SignPayload("k", body)
	if !VerifyPayload("k", body, sig) {
		t.Fatal("valid signature rejected")
	}
	for _, bad := range []string{"", "sha256=", sig[len("sha256="):], SignPayload("other", body), "sha1=" + sig[len("sha256="):]} {
		if VerifyPayload("k", body, bad) {
			t.Fatalf("invalid signature %q accepted", bad)
		}
	}
	if VerifyPayload("k", []byte(`{"a":2}`), sig) {
		t.Fatal("tampered body accepted")
	}
	if VerifyPayload("", body, sig) {
		t.Fatal("empty secret accepted")
	}
}

func TestApplySignature_NoSecretNoHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	applySignature(req, "", []byte("x"), time.Now())
	if req.Header.Get(SignatureHeader) != "" || req.Header.Get(SignatureHeaderAlias) != "" || req.Header.Get(TimestampHeader) != "" {
		t.Fatalf("headers set without secret: %v", req.Header)
	}
}

type captured struct {
	body    []byte
	headers http.Header
}

func captureServer(t *testing.T) (*httptest.Server, chan captured) {
	t.Helper()
	ch := make(chan captured, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		ch <- captured{body: b, headers: r.Header.Clone()}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, ch
}

func TestSendWebhook_SignsWithoutChangingBody(t *testing.T) {
	srv, ch := captureServer(t)
	p := &webhookProducer{secret: "global"}
	body := []byte("{\"event\":\"Message\",  \"data\":{\"x\":\"\\u00e9\"}}\n") // odd spacing/escapes must survive byte-for-byte

	before := time.Now().Unix()
	err, _, status := p.sendWebhook(srv.URL, body, p.secretFor("per-instance"), "inst")
	if err != nil || status != http.StatusOK {
		t.Fatalf("sendWebhook: err=%v status=%d", err, status)
	}
	got := <-ch
	if string(got.body) != string(body) {
		t.Fatalf("body changed: %q", got.body)
	}
	sig := got.headers.Get(SignatureHeader)
	if sig != SignPayload("per-instance", body) {
		t.Fatalf("%s = %q", SignatureHeader, sig)
	}
	if got.headers.Get(SignatureHeaderAlias) != sig {
		t.Fatalf("%s = %q", SignatureHeaderAlias, got.headers.Get(SignatureHeaderAlias))
	}
	ts, err := strconv.ParseInt(got.headers.Get(TimestampHeader), 10, 64)
	if err != nil || ts < before || ts > time.Now().Unix() {
		t.Fatalf("%s = %q", TimestampHeader, got.headers.Get(TimestampHeader))
	}
	if got.headers.Get("Content-Type") != "application/json" {
		t.Fatalf("content-type = %q", got.headers.Get("Content-Type"))
	}
}

func TestSendWebhook_UnsignedWhenNoSecret(t *testing.T) {
	srv, ch := captureServer(t)
	p := &webhookProducer{}
	if err, _, _ := p.sendWebhook(srv.URL, []byte(`{}`), p.secretFor(""), "inst"); err != nil {
		t.Fatal(err)
	}
	got := <-ch
	if got.headers.Get(SignatureHeader) != "" || got.headers.Get(TimestampHeader) != "" {
		t.Fatalf("unexpected signature headers: %v", got.headers)
	}
}

func TestSecretFor_FallsBackToGlobal(t *testing.T) {
	p := &webhookProducer{secret: "global"}
	if p.secretFor("") != "global" || p.secretFor("inst") != "inst" {
		t.Fatal("secretFor precedence wrong")
	}
}
