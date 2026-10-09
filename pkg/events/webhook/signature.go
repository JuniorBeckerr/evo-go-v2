package webhook_producer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"
)

const (
	// SignatureHeader carries "sha256=<hex HMAC-SHA256(secret, raw body)>".
	SignatureHeader = "X-Evo-Signature"
	// SignatureHeaderAlias carries the same value under the generic name used by
	// other providers (e.g. ms-wpp's X-Webhook-Signature verifier).
	SignatureHeaderAlias = "X-Webhook-Signature"
	// TimestampHeader carries the unix time (seconds) of the delivery attempt.
	// It is informational: it is NOT part of the signed payload.
	TimestampHeader = "X-Evo-Timestamp"
	// SignaturePrefix precedes the hex digest in the signature headers.
	SignaturePrefix = "sha256="
)

// SignPayload returns "sha256=<hex>" where hex is HMAC-SHA256(secret, body).
// The body bytes are signed exactly as they are sent.
func SignPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return SignaturePrefix + hex.EncodeToString(mac.Sum(nil))
}

// VerifyPayload reports whether signature ("sha256=<hex>") matches body under secret.
func VerifyPayload(secret string, body []byte, signature string) bool {
	if secret == "" || len(signature) <= len(SignaturePrefix) || signature[:len(SignaturePrefix)] != SignaturePrefix {
		return false
	}
	return hmac.Equal([]byte(signature), []byte(SignPayload(secret, body)))
}

// applySignature sets the signature/timestamp headers on req when secret is
// non-empty. It never touches the request body.
func applySignature(req *http.Request, secret string, body []byte, now time.Time) {
	if secret == "" {
		return
	}
	sig := SignPayload(secret, body)
	req.Header.Set(SignatureHeader, sig)
	req.Header.Set(SignatureHeaderAlias, sig)
	req.Header.Set(TimestampHeader, strconv.FormatInt(now.Unix(), 10))
}
