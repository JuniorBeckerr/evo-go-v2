package instance_service

import (
	"encoding/json"
	"testing"
)

func TestQrcodeStructJSONKeepsLegacyAndUpstreamKeys(t *testing.T) {
	b, err := json.Marshal(&QrcodeStruct{Qrcode: "data:image/png;base64,AAA|ref", Code: "ref"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{
		"Qrcode": "data:image/png;base64,AAA|ref", "Code": "ref",
		"qrcode": "data:image/png;base64,AAA|ref", "code": "ref",
	} {
		if got[k] != want {
			t.Fatalf("key %q = %v, want %q (json: %s)", k, got[k], want, b)
		}
	}
	if _, ok := got["passkeyStage"]; ok {
		t.Fatalf("passkey fields must be omitted when empty: %s", b)
	}
}

func TestQrcodeStructJSONPasskey(t *testing.T) {
	b, _ := json.Marshal(QrcodeStruct{PasskeyStage: "awaiting", PasskeyOpenURL: "https://web.whatsapp.com/#wapk=x"})
	var got map[string]any
	_ = json.Unmarshal(b, &got)
	if got["passkeyStage"] != "awaiting" || got["passkeyOpenUrl"] != "https://web.whatsapp.com/#wapk=x" {
		t.Fatalf("unexpected json: %s", b)
	}
}
