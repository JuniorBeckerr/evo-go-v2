package send_service

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
)

func TestBuildNativeFlowList_SingleSelectButton(t *testing.T) {
	data := &ListStruct{
		Title: "Planos", Description: "Escolha", ButtonText: "Ver opcoes", FooterText: "rodape",
		Sections: []Section{{Title: "Secao 1", Rows: []Row{
			{Title: "Item \"1\"", Description: "desc 1", RowId: "r1"},
			{Title: "Item 2", RowId: "r2"},
		}}},
	}
	msg, err := buildNativeFlowList(data, "123")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if msg.ListMessage != nil || msg.DocumentWithCaptionMessage != nil {
		t.Fatal("lista nao deve usar o ListMessage legado (WhatsApp devolve 405)")
	}
	im := msg.GetInteractiveMessage()
	if im == nil {
		t.Fatal("esperava InteractiveMessage")
	}
	if im.GetHeader().GetTitle() != "Planos" || im.GetBody().GetText() != "Escolha" || im.GetFooter().GetText() != "rodape" {
		t.Fatalf("cabecalho/corpo/rodape errados: %v", im)
	}
	flow := im.GetNativeFlowMessage()
	if flow.GetMessageVersion() != 1 || len(flow.GetButtons()) != 1 {
		t.Fatalf("native flow invalido: %v", flow)
	}
	btn := flow.GetButtons()[0]
	if btn.GetName() != "single_select" {
		t.Fatalf("botao deve ser single_select, veio %q", btn.GetName())
	}
	var params struct {
		Title    string `json:"title"`
		Sections []struct {
			Title string `json:"title"`
			Rows  []struct {
				Header, Title, Description, ID string
			} `json:"rows"`
		} `json:"sections"`
	}
	if err := json.Unmarshal([]byte(btn.GetButtonParamsJSON()), &params); err != nil {
		t.Fatalf("buttonParamsJSON nao e JSON: %v", err)
	}
	if params.Title != "Ver opcoes" || len(params.Sections) != 1 || len(params.Sections[0].Rows) != 2 {
		t.Fatalf("parametros errados: %+v", params)
	}
	row := params.Sections[0].Rows[0]
	if row.Title != "Item \"1\"" || row.ID != "r1" || row.Description != "desc 1" || row.Header != "" {
		t.Fatalf("linha errada: %+v", row)
	}
}

func TestBuildNativeFlowList_RequiresRows(t *testing.T) {
	if _, err := buildNativeFlowList(&ListStruct{Description: "x", Sections: []Section{{Title: "s"}}}, "1"); err == nil {
		t.Fatal("lista sem linhas deve falhar")
	}
}

func TestCarouselButton_EscapesJSON(t *testing.T) {
	cases := []struct {
		in       CarouselButtonStruct
		name     string
		key, val string
	}{
		{CarouselButtonStruct{Type: "url", DisplayText: `Abrir "site"`, Id: "https://x.com/?a=1&b=2"}, "cta_url", "url", "https://x.com/?a=1&b=2"},
		{CarouselButtonStruct{Type: "COPY", DisplayText: "Copiar", CopyCode: `CUPOM"10`}, "cta_copy", "copy_code", `CUPOM"10`},
		{CarouselButtonStruct{Type: "CALL", DisplayText: "Ligar", Id: "+5511999999999"}, "cta_call", "phone_number", "+5511999999999"},
		{CarouselButtonStruct{Type: "", DisplayText: "Quero", Id: "c1"}, "quick_reply", "id", "c1"},
	}
	for _, c := range cases {
		b := carouselButton(c.in)
		if b.GetName() != c.name {
			t.Fatalf("%v: nome %q", c.in, b.GetName())
		}
		var params map[string]string
		if err := json.Unmarshal([]byte(b.GetButtonParamsJSON()), &params); err != nil {
			t.Fatalf("%v: JSON invalido %q: %v", c.in, b.GetButtonParamsJSON(), err)
		}
		if params[c.key] != c.val || params["display_text"] != c.in.DisplayText {
			t.Fatalf("%v: parametros %v", c.in, params)
		}
	}
}

func TestMergeLinkMetadata_KeepsCallerFields(t *testing.T) {
	data := &LinkStruct{Title: "Meu titulo", Description: ""}
	mergeLinkMetadata(data, "Titulo da pagina", "Descricao da pagina", "https://img")
	if data.Title != "Meu titulo" || data.Description != "Descricao da pagina" || data.ImgUrl != "https://img" {
		t.Fatalf("merge errado: %+v", data)
	}
}

func TestLinkMatchedText_FallsBackToUrl(t *testing.T) {
	data := &LinkStruct{Text: "veja isto", Url: "https://github.com"}
	if got := linkMatchedText(data); got != "https://github.com" {
		t.Fatalf("matchedText %q", got)
	}
	if data.Text != "veja isto\nhttps://github.com" {
		t.Fatalf("o link deve ir no texto para o WhatsApp mostrar a previa: %q", data.Text)
	}
	data = &LinkStruct{Text: "veja https://a.com/x", Url: "https://github.com"}
	if got := linkMatchedText(data); got != "https://a.com/x" || data.Text != "veja https://a.com/x" {
		t.Fatalf("com URL no texto usa a do texto: %q %q", got, data.Text)
	}
}

func TestAudioIsPTT_DefaultTrue(t *testing.T) {
	f := false
	if !audioIsPTT(&MediaStruct{Type: "audio"}) {
		t.Fatal("sem o campo ptt o audio continua sendo de voz (compatibilidade)")
	}
	if audioIsPTT(&MediaStruct{Type: "audio", Ptt: &f}) {
		t.Fatal("ptt=false deve mandar audio comum")
	}
}

func TestBuildAudioMessage_CommonAudioHasNoVoiceFields(t *testing.T) {
	m := buildAudioMessage(false, 7)
	if m.GetPTT() || m.Waveform != nil || m.StreamingSidecar != nil || m.GetSeconds() != 7 {
		t.Fatalf("audio comum errado: %v", m)
	}
	v := buildAudioMessage(true, 3)
	if !v.GetPTT() || v.Waveform == nil || v.GetSeconds() != 3 {
		t.Fatalf("audio de voz errado: %v", v)
	}
}

func TestVideoPreview_ThumbnailAndDuration(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg ausente")
	}
	out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i",
		"testsrc=size=320x240:rate=10:duration=2", "-c:v", "mpeg4", "-f", "mp4", "-movflags", "frag_keyframe+empty_moov",
		"pipe:1").Output()
	if err != nil {
		t.Fatalf("gerar video: %v", err)
	}
	p := videoPreview(out)
	if len(p.Thumbnail) == 0 || p.Seconds != 2 || p.Width != 320 || p.Height != 240 {
		t.Fatalf("previa errada: thumb=%d s=%d %dx%d", len(p.Thumbnail), p.Seconds, p.Width, p.Height)
	}
}

func TestVideoPreview_InvalidDataIsEmpty(t *testing.T) {
	p := videoPreview([]byte("nao e video"))
	if p.Thumbnail != nil || p.Seconds != 0 {
		t.Fatalf("dado invalido deve dar previa vazia: %+v", p)
	}
}

func TestParseLinkMetadata_PrefersOGAndIgnoresSVGTitles(t *testing.T) {
	page := `<html><head><title>GitHub · Build</title><meta property="og:title" content="GitHub OG">
<meta name="description" content="desc"><meta property="og:image" content="https://img/x.png"></head>
<body><svg><title>Vodafone</title></svg></body></html>`
	title, desc, img, err := parseLinkMetadata(strings.NewReader(page))
	if err != nil || title != "GitHub OG" || desc != "desc" || img != "https://img/x.png" {
		t.Fatalf("got %q %q %q %v", title, desc, img, err)
	}
	title, _, _, _ = parseLinkMetadata(strings.NewReader(`<html><head><title>Pagina</title></head><body><svg><title>Vodafone</title></svg></body></html>`))
	if title != "Pagina" {
		t.Fatalf("titulo deve ser o <title> do head, veio %q", title)
	}
}

func TestLinkMatchedText_HideUrlKeepsTextClean(t *testing.T) {
	data := &LinkStruct{Text: "toque no cartao", Url: "https://s.x/l/abc", HideUrl: true}
	if got := linkMatchedText(data); got != "https://s.x/l/abc" || data.Text != "toque no cartao" {
		t.Fatalf("hideUrl: matched=%q text=%q", got, data.Text)
	}
}

func TestBuildListMessage_Formats(t *testing.T) {
	data := &ListStruct{Title: "T", Description: "D", ButtonText: "B", FooterText: "F",
		Sections: []Section{{Title: "S", Rows: []Row{{Title: "R", RowId: "r"}}}}}
	cases := map[string]func(m *waE2E.Message) bool{
		"": func(m *waE2E.Message) bool {
			return m.GetViewOnceMessage().GetMessage().GetInteractiveMessage() != nil
		},
		"interactive": func(m *waE2E.Message) bool {
			return m.GetInteractiveMessage() != nil && m.GetMessageContextInfo().GetDeviceListMetadata() != nil
		},
		"interactive_viewonce": func(m *waE2E.Message) bool {
			in := m.GetViewOnceMessage().GetMessage()
			return in.GetInteractiveMessage() != nil && in.GetMessageContextInfo().GetDeviceListMetadataVersion() == 2
		},
		"legacy_viewonce": func(m *waE2E.Message) bool {
			return m.GetViewOnceMessage().GetMessage().GetListMessage().GetListType() == waE2E.ListMessage_SINGLE_SELECT
		},
		"legacy": func(m *waE2E.Message) bool {
			return m.GetListMessage().GetListType() == waE2E.ListMessage_SINGLE_SELECT && m.GetViewOnceMessage() == nil
		},
	}
	for format, ok := range cases {
		data.Format = format
		msg, msgType, err := buildListMessage(data, "1")
		if err != nil || !ok(msg) {
			t.Fatalf("formato %q: tipo=%s err=%v msg=%v", format, msgType, err, msg)
		}
		wantType := "InteractiveMessage"
		if strings.HasPrefix(format, "legacy") {
			wantType = "ListMessage"
		}
		if msgType != wantType {
			t.Fatalf("formato %q: tipo %s", format, msgType)
		}
	}
}
