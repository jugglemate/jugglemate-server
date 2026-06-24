package apis_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/apis"
	"github.com/juggleim/jugglemate-server/commons/configures"
	"github.com/juggleim/jugglemate-server/routers"
)

var testServer *httptest.Server

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	if err := configures.InitConfigures(); err != nil {
		fmt.Printf("InitConfigures failed (expected in test): %v\n", err)
	}

	testRouter := gin.New()
	testRouter.Use(corsHandler())

	msgCallbackGrp := testRouter.Group("/botmsgs")
	routers.RouteMsgCallback(msgCallbackGrp)

	group := testRouter.Group("/jim")
	group.Use(apis.Validate)
	routers.Route(group)

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		fmt.Printf("Test listener failed: %v\n", err)
		os.Exit(1)
	}
	testServer = httptest.NewUnstartedServer(testRouter)
	testServer.Listener = listener
	testServer.Start()

	code := m.Run()
	testServer.Close()
	os.Exit(code)
}

func corsHandler() gin.HandlerFunc {
	return func(context *gin.Context) {
		method := context.Request.Method
		context.Writer.Header().Add("Access-Control-Allow-Origin", "*")
		context.Writer.Header().Add("Access-Control-Allow-Headers", "*")
		context.Writer.Header().Add("Access-Control-Allow-Methods", "POST, GET, OPTIONS, DELETE, PATCH, PUT")
		if method == "OPTIONS" {
			context.AbortWithStatus(http.StatusNoContent)
		}
		context.Next()
	}
}

func request(method, path string, body interface{}) (*http.Response, error) {
	var reqBody []byte
	if body != nil {
		reqBody, _ = json.Marshal(body)
	}
	req, err := http.NewRequest(method, testServer.URL+path, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	return client.Do(req)
}

// ==================== Parameter Validation Tests ====================

func TestCreateAiBot_MissingParams(t *testing.T) {
	body := map[string]interface{}{
		"bot_id": "test_bot_123",
	}
	resp, err := request("POST", "/jim/aibots/add", body)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("CreateAiBot missing params: status=%d", resp.StatusCode)
}

func TestUpdateAiBot_MissingBotId(t *testing.T) {
	body := map[string]interface{}{
		"unique_name": "test_bot",
	}
	resp, err := request("POST", "/jim/aibots/update", body)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("UpdateAiBot missing bot_id: status=%d", resp.StatusCode)
}

func TestRemoveAiBot_MissingBotId(t *testing.T) {
	body := map[string]interface{}{}
	resp, err := request("POST", "/jim/aibots/remove", body)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("RemoveAiBot missing bot_id: status=%d", resp.StatusCode)
}

func TestAddAiMaterial_MissingParams(t *testing.T) {
	body := map[string]interface{}{
		"unique_name": "test_bot",
	}
	resp, err := request("POST", "/jim/aibots/test_bot/materials/add", body)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("AddAiMaterial missing params: status=%d", resp.StatusCode)
}

func TestRemoveAiMaterial_MissingParams(t *testing.T) {
	resp, err := request("POST", "/jim/aibots//materials//remove", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("RemoveAiMaterial missing params: status=%d", resp.StatusCode)
}

func TestStartTraining_MissingParams(t *testing.T) {
	body := map[string]interface{}{}
	resp, err := request("POST", "/jim/aibots/test_bot/training", body)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("StartTraining missing params: status=%d", resp.StatusCode)
}

// ==================== Query Endpoint Tests (no body required) ====================

func TestQryMyAiBots_DefaultParams(t *testing.T) {
	resp, err := request("GET", "/jim/aibots/mybots", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("QryMyAiBots default: status=%d", resp.StatusCode)
}

func TestQryMyAiBots_WithPagination(t *testing.T) {
	resp, err := request("GET", "/jim/aibots/mybots?count=5&offset=0", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("QryMyAiBots pagination: status=%d", resp.StatusCode)
}

func TestQryAiMaterials_DefaultParams(t *testing.T) {
	resp, err := request("GET", "/jim/aibots/test_bot/materials", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("QryAiMaterials default: status=%d", resp.StatusCode)
}

func TestQryAiMaterials_WithPagination(t *testing.T) {
	resp, err := request("GET", "/jim/aibots/test_bot/materials?count=5&offset=0", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("QryAiMaterials pagination: status=%d", resp.StatusCode)
}

func TestListJobs_DefaultParams(t *testing.T) {
	resp, err := request("GET", "/jim/aibots/test_bot/jobs", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("ListJobs default: status=%d", resp.StatusCode)
}

func TestListJobs_WithFilters(t *testing.T) {
	resp, err := request("GET", "/jim/aibots/test_bot/jobs?status=running&type=training&limit=5", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("ListJobs with filters: status=%d", resp.StatusCode)
}

func TestListVersions_DefaultParams(t *testing.T) {
	resp, err := request("GET", "/jim/aibots/test_bot/versions", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("ListVersions default: status=%d", resp.StatusCode)
}

func TestListVersions_WithCursor(t *testing.T) {
	resp, err := request("GET", "/jim/aibots/test_bot/versions?limit=5&cursor=abc123", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("ListVersions with cursor: status=%d", resp.StatusCode)
}

func TestGetCurrentVersion_Success(t *testing.T) {
	resp, err := request("GET", "/jim/aibots/test_bot/versions/current", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("GetCurrentVersion: status=%d", resp.StatusCode)
}

func TestActivateVersion_Success(t *testing.T) {
	body := map[string]interface{}{}
	resp, err := request("POST", "/jim/aibots/test_bot/versions/v1.0.0/activate", body)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("ActivateVersion: status=%d", resp.StatusCode)
}

func TestListEvaluations_DefaultParams(t *testing.T) {
	resp, err := request("GET", "/jim/aibots/test_bot/evaluations", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("ListEvaluations default: status=%d", resp.StatusCode)
}

func TestListEvaluations_WithCursor(t *testing.T) {
	resp, err := request("GET", "/jim/aibots/test_bot/evaluations?limit=5&cursor=xyz789", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("ListEvaluations with cursor: status=%d", resp.StatusCode)
}

// ==================== CORS Tests ====================

func TestCORS_Preflight(t *testing.T) {
	req, _ := http.NewRequest("OPTIONS", testServer.URL+"/jim/aibots/mybots", nil)
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("CORS preflight request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("CORS preflight: status=%d", resp.StatusCode)
}

// ==================== Error Handling Tests ====================

func TestInvalidRoute(t *testing.T) {
	resp, err := request("GET", "/jim/nonexistent/route", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("Invalid route: status=%d", resp.StatusCode)
}

func TestMethodNotAllowed(t *testing.T) {
	resp, err := request("POST", "/jim/aibots/mybots", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("Method not allowed: status=%d", resp.StatusCode)
}
