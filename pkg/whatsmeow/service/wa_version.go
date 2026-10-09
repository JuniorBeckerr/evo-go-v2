package whatsmeow_service

import "go.mau.fi/whatsmeow/store"

// MinWAVersion is the minimum WhatsApp Web client version announced to the server.
// The official whatsmeow pins an older default (2.3000.1042386815); the fork used to
// bump it inside the vendored whatsmeow-lib. Now that the official library is used,
// the bump is applied here at startup. The dynamic version fetched from
// web.whatsapp.com (see fetchWhatsAppWebVersion) still overrides it per client.
var MinWAVersion = store.WAVersionContainer{2, 3000, 1044083468}

func init() {
	EnsureMinWAVersion()
}

// EnsureMinWAVersion raises whatsmeow's global client version to MinWAVersion
// when the library default is older. It never lowers a newer version.
func EnsureMinWAVersion() {
	if store.GetWAVersion().LessThan(MinWAVersion) {
		store.SetWAVersion(MinWAVersion)
	}
}
