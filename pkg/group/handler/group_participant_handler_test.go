package group_handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	group_service "github.com/evolution-foundation/evolution-go/pkg/group/service"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	"github.com/gin-gonic/gin"
)

type fakeParticipantService struct {
	group_service.GroupService
	results []group_service.ParticipantResult
}

func (f *fakeParticipantService) UpdateParticipant(_ *group_service.AddParticipantStruct, _ *instance_model.Instance) ([]group_service.ParticipantResult, error) {
	return f.results, nil
}

func TestUpdateParticipant_ReturnsPerParticipantResults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeParticipantService{results: []group_service.ParticipantResult{
		{JID: "554599242689@s.whatsapp.net", Status: 200},
		{JID: "5511900000001@s.whatsapp.net", Status: 403, Error: 403,
			AddRequest: &group_service.ParticipantAddRequest{Code: "ABCD", Expiration: 1791700000}},
	}}
	r := gin.New()
	r.POST("/group/participant", func(c *gin.Context) {
		c.Set("instance", &instance_model.Instance{Id: "inst-1"})
		c.Next()
	}, NewGroupHandler(svc).UpdateParticipant)
	body := `{"groupJid":"120363000000000000@g.us","action":"add","participants":["554599242689","5511900000001"]}`
	req := httptest.NewRequest(http.MethodPost, "/group/participant", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Message      string `json:"message"`
		Participants []struct {
			JID        string `json:"jid"`
			Status     int    `json:"status"`
			Error      int    `json:"error"`
			AddRequest *struct {
				Code       string `json:"code"`
				Expiration int64  `json:"expiration"`
			} `json:"addRequest"`
		} `json:"participants"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Message != "success" || len(resp.Participants) != 2 {
		t.Fatalf("body = %s", w.Body.String())
	}
	p := resp.Participants[1]
	if p.Status != 403 || p.AddRequest == nil || p.AddRequest.Code != "ABCD" || p.AddRequest.Expiration != 1791700000 {
		t.Fatalf("participant = %#v", p)
	}
}
