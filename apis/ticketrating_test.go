package apis

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/services"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

type stubRatingStorage struct {
	out *storageModels.TicketRating
}

func (s *stubRatingStorage) Create(item storageModels.TicketRating) error { return nil }
func (s *stubRatingStorage) FindByTicketAndCustomer(appkey, ticketId, customerId string) (*storageModels.TicketRating, error) {
	if s.out != nil && s.out.AppKey == appkey && s.out.TicketId == ticketId && s.out.CustomerId == customerId {
		cp := *s.out
		return &cp, nil
	}
	return nil, nil
}
func (s *stubRatingStorage) QryCsatByAssigneeAndTime(appkey string, startMs, endMs int64) (map[string]storageModels.CsatAggregate, error) {
	return nil, nil
}

func TestQryTicketRatingHandlerOK(t *testing.T) {
	rating := &storageModels.TicketRating{
		AppKey:      "app_1",
		TicketId:    "ticket_1",
		CustomerId:  "customer_1",
		AssigneeId:  "u_seat_1",
		Rating:      5,
		Comment:     "好评",
		CreatedTime: 1785000000000,
	}
	services.OverrideTestRatingStorage = func() storageModels.ITicketRatingStorage { return &stubRatingStorage{out: rating} }
	defer func() { services.OverrideTestRatingStorage = nil }()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/tickets/ticket_1/rating?customer_id=customer_1", nil)
	ctx.Params = gin.Params{{Key: "ticket_id", Value: "ticket_1"}}
	ctx.Set(string(ctxs.CtxKey_AppKey), "app_1")

	QryTicketRating(ctx)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"rating":5`) {
		t.Fatalf("body = %s", w.Body.String())
	}
}

// 占用符，防止 "imported and not used"。
var _ = services.SetCSATCallerForTest
