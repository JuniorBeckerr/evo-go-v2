package group_handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	group_service "github.com/evolution-foundation/evolution-go/pkg/group/service"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	auth_middleware "github.com/evolution-foundation/evolution-go/pkg/middleware"
	"github.com/gin-gonic/gin"
)

// fakeCreateGroupService implements only CreateGroup; any other method panics
// through the nil embedded interface.
type fakeCreateGroupService struct {
	group_service.GroupService
	got *group_service.CreateGroupStruct
}

func (f *fakeCreateGroupService) CreateGroup(data *group_service.CreateGroupStruct, _ *instance_model.Instance) (gin.H, error) {
	f.got = data
	return gin.H{"jid": "120363000000000000@g.us"}, nil
}

// postCreate runs the same chain as routes.go: ValidateMultipleNumbers("participants") + CreateGroup.
func postCreate(t *testing.T, svc *fakeCreateGroupService, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewGroupHandler(svc)
	r.POST("/group/create", func(c *gin.Context) {
		c.Set("instance", &instance_model.Instance{Id: "inst-1"})
		c.Next()
	}, auth_middleware.NewJIDValidationMiddleware().ValidateMultipleNumbers("participants"), h.CreateGroup)
	req := httptest.NewRequest(http.MethodPost, "/group/create", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCreateGroup_WithoutParticipantsIsAccepted(t *testing.T) {
	// Group with only the creator: whatsmeow CreateGroup accepts an empty list.
	for _, body := range []string{
		`{"groupName":"Solo","participants":[]}`,
		`{"groupName":"Solo"}`,
	} {
		svc := &fakeCreateGroupService{}
		w := postCreate(t, svc, body)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, body = %s", body, w.Code, w.Body.String())
		}
		if svc.got == nil || svc.got.GroupName != "Solo" || len(svc.got.Participants) != 0 {
			t.Fatalf("%s: service got %#v", body, svc.got)
		}
	}
}

func TestCreateGroup_ParticipantsAreNormalized(t *testing.T) {
	svc := &fakeCreateGroupService{}
	w := postCreate(t, svc, `{"groupName":"G","participants":["5511999998888"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if svc.got == nil || len(svc.got.Participants) != 1 || !strings.HasSuffix(svc.got.Participants[0], "5511999998888@s.whatsapp.net") {
		t.Fatalf("service got %#v", svc.got)
	}
}

func TestCreateGroup_ValidationErrors(t *testing.T) {
	cases := map[string]string{
		`{"participants":[]}`:                          "groupName is required",
		`{"groupName":"G","participants":["abc@x@y"]}`: "Invalid participants[0] format",
		`not json`: "Invalid JSON format",
	}
	for body, want := range cases {
		svc := &fakeCreateGroupService{}
		w := postCreate(t, svc, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400 (body %s)", body, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("%s: body = %s, want %q", body, w.Body.String(), want)
		}
		if svc.got != nil {
			t.Fatalf("%s: service should not be called", body)
		}
	}
}
