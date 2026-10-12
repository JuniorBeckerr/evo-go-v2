package send_service

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// buildNativeFlowList monta a lista como InteractiveMessage com um botao NativeFlow "single_select" (mesmo caminho
// dos botoes, que renderiza no Android, iOS e Web). O ListMessage legado embrulhado em DocumentWithCaptionMessage
// com o node <bot biz_bot> e recusado pelo servidor do WhatsApp com erro 405 para contas comuns.
func buildNativeFlowList(data *ListStruct, templateID string) (*waE2E.Message, error) {
	type row struct {
		Header      string `json:"header"`
		Title       string `json:"title"`
		Description string `json:"description"`
		ID          string `json:"id"`
	}
	type section struct {
		Title          string `json:"title"`
		HighlightLabel string `json:"highlight_label"`
		Rows           []row  `json:"rows"`
	}
	buttonText := data.ButtonText
	if buttonText == "" {
		buttonText = "Ver Menu"
	}
	sections := []section{}
	total := 0
	for si, s := range data.Sections {
		rows := []row{}
		for ri, r := range s.Rows {
			id := r.RowId
			if id == "" {
				id = "row_" + strconv.Itoa(si) + "_" + strconv.Itoa(ri)
			}
			title := r.Title
			if title == "" {
				title = " "
			}
			rows = append(rows, row{Title: title, Description: r.Description, ID: id})
		}
		total += len(rows)
		sections = append(sections, section{Title: s.Title, Rows: rows})
	}
	if total == 0 {
		return nil, errors.New("lista exige ao menos uma linha")
	}
	params, err := json.Marshal(map[string]interface{}{"title": buttonText, "sections": sections})
	if err != nil {
		return nil, err
	}
	messageParams := `{"from":"api","templateId":` + templateID + `}`
	im := &waE2E.InteractiveMessage{
		Body: &waE2E.InteractiveMessage_Body{Text: proto.String(data.Description)},
		InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
			NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
				Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{{
					Name:             proto.String("single_select"),
					ButtonParamsJSON: proto.String(string(params)),
				}},
				MessageParamsJSON: proto.String(messageParams),
				MessageVersion:    proto.Int32(1),
			},
		},
		ContextInfo: &waE2E.ContextInfo{},
	}
	if data.Title != "" {
		im.Header = &waE2E.InteractiveMessage_Header{Title: proto.String(data.Title), HasMediaAttachment: proto.Bool(false)}
	}
	if data.FooterText != "" {
		im.Footer = &waE2E.InteractiveMessage_Footer{Text: proto.String(data.FooterText)}
	}
	return &waE2E.Message{InteractiveMessage: im}, nil
}

// carouselButton converte o botao do cartao em NativeFlowButton com JSON escapado (o Sprintf antigo quebrava o
// JSON com aspas no texto). URL e telefone vem no `id` (CarouselButtonStruct).
func carouselButton(btn CarouselButtonStruct) *waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton {
	var name string
	var params map[string]string
	switch strings.ToUpper(btn.Type) {
	case "URL":
		name, params = "cta_url", map[string]string{"display_text": btn.DisplayText, "url": btn.Id, "merchant_url": btn.Id}
	case "CALL":
		name, params = "cta_call", map[string]string{"display_text": btn.DisplayText, "phone_number": btn.Id}
	case "COPY":
		name, params = "cta_copy", map[string]string{"display_text": btn.DisplayText, "copy_code": btn.CopyCode}
	default:
		name, params = "quick_reply", map[string]string{"display_text": btn.DisplayText, "id": btn.Id}
	}
	b, _ := json.Marshal(params)
	return &waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
		Name:             proto.String(name),
		ButtonParamsJSON: proto.String(string(b)),
	}
}

// mergeLinkMetadata completa so os campos que o chamador deixou vazios com os metadados da pagina.
func mergeLinkMetadata(data *LinkStruct, title, description, imgURL string) {
	if data.Title == "" {
		data.Title = title
	}
	if data.Description == "" {
		data.Description = description
	}
	if data.ImgUrl == "" {
		data.ImgUrl = imgURL
	}
}

// linkMatchedText devolve a URL da previa: a do texto ou, sem URL no texto, o campo `url` (anexado ao texto para o
// WhatsApp mostrar a previa).
func linkMatchedText(data *LinkStruct) string {
	if found := findURL(data.Text); found != "" {
		return found
	}
	if data.Url == "" {
		return ""
	}
	if strings.TrimSpace(data.Text) == "" {
		data.Text = data.Url
	} else {
		data.Text = data.Text + "\n" + data.Url
	}
	return data.Url
}

// linkHTTPClient: busca de metadados da previa com timeout (http.Get sem timeout podia travar o envio).
var linkHTTPClient = &http.Client{Timeout: 10 * time.Second}

// audioIsPTT: sem o campo `ptt` o audio continua indo como mensagem de voz (compatibilidade com quem ja usa).
func audioIsPTT(data *MediaStruct) bool {
	return data.Ptt == nil || *data.Ptt
}

const (
	pttStreamingSidecar = "QpmXDsU7YLagdg=="
	pttWaveform         = "OjAnExISDgsKCAkJBwgkHAQEBBEFAwMNAxAcKCgkFzM0QUE4Jh4eKAoKChcLCwkeFgkJCQo3JiQmIiIRPz8/Ow=="
)

// buildAudioMessage devolve os campos de apresentacao do AudioMessage: voz (PTT, com waveform) ou audio comum.
func buildAudioMessage(ptt bool, seconds int) *waE2E.AudioMessage {
	m := &waE2E.AudioMessage{PTT: proto.Bool(ptt), Seconds: proto.Uint32(uint32(seconds))}
	if ptt {
		m.StreamingSidecar = []byte(pttStreamingSidecar)
		m.Waveform = []byte(pttWaveform)
	}
	return m
}

type videoPreviewInfo struct {
	Thumbnail     []byte
	Seconds       uint32
	Width, Height uint32
}

// videoPreview extrai miniatura JPEG (1o quadro), duracao e dimensoes com ffmpeg/ffprobe. Sem isso o WhatsApp mostra
// o video sem previa e com 0:00. Falha = previa vazia (o video segue sem ela).
func videoPreview(fileData []byte) videoPreviewInfo {
	var info videoPreviewInfo
	tmp, err := os.CreateTemp("", "video-*.mp4")
	if err != nil {
		return info
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(fileData); err != nil {
		tmp.Close()
		return info
	}
	tmp.Close()

	probe := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries",
		"stream=width,height:format=duration", "-of", "json", tmp.Name())
	if out, err := runWithTimeout(probe, 15*time.Second); err == nil {
		var parsed struct {
			Streams []struct {
				Width  uint32 `json:"width"`
				Height uint32 `json:"height"`
			} `json:"streams"`
			Format struct {
				Duration string `json:"duration"`
			} `json:"format"`
		}
		if json.Unmarshal(out, &parsed) == nil && len(parsed.Streams) > 0 {
			info.Width, info.Height = parsed.Streams[0].Width, parsed.Streams[0].Height
			if d, err := strconv.ParseFloat(parsed.Format.Duration, 64); err == nil {
				info.Seconds = uint32(d + 0.5)
			}
		}
	}
	if info.Width == 0 {
		return videoPreviewInfo{}
	}
	thumb := exec.Command("ffmpeg", "-v", "error", "-i", tmp.Name(), "-frames:v", "1", "-vf", "scale=200:-2",
		"-f", "image2", "-c:v", "mjpeg", "-q:v", "5", "pipe:1")
	if out, err := runWithTimeout(thumb, 15*time.Second); err == nil && len(out) > 0 {
		info.Thumbnail = out
	}
	return info
}

func runWithTimeout(cmd *exec.Cmd, timeout time.Duration) ([]byte, error) {
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return out.Bytes(), err
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		<-done
		return nil, errors.New("timeout")
	}
}

// applyMediaPresentation ajusta a apresentacao da midia ja montada: audio de voz x audio comum (campo `ptt`) e
// miniatura/duracao/dimensoes do video.
func applyMediaPresentation(media *waE2E.Message, data *MediaStruct, fileData []byte) {
	if media == nil {
		return
	}
	if a := media.GetAudioMessage(); a != nil {
		p := buildAudioMessage(audioIsPTT(data), int(a.GetSeconds()))
		a.PTT, a.Waveform, a.StreamingSidecar = p.PTT, p.Waveform, p.StreamingSidecar
	}
	if v := media.GetVideoMessage(); v != nil {
		info := videoPreview(fileData)
		if len(info.Thumbnail) > 0 {
			v.JPEGThumbnail = info.Thumbnail
		}
		if info.Seconds > 0 {
			v.Seconds = proto.Uint32(info.Seconds)
		}
		if info.Width > 0 && info.Height > 0 {
			v.Width, v.Height = proto.Uint32(info.Width), proto.Uint32(info.Height)
		}
	}
}
