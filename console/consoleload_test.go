package console

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLoadConsoleWebServesIndexAndClientRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	LoadConsoleWeb(router)

	for _, path := range []string{ConsoleBasePath, ConsoleBasePath + "/", ConsoleBasePath + "/users"} {
		resp := requestConsole(router, path)
		if resp.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", path, resp.Code)
		}
		body := resp.Body.String()
		if !strings.Contains(body, `<div id="root"></div>`) {
			t.Fatalf("GET %s did not return console index", path)
		}
		if !strings.Contains(body, `/jmateconsole/assets/`) {
			t.Fatalf("GET %s index does not reference /jmateconsole assets", path)
		}
	}
}

func TestLoadConsoleWebServesEmbeddedAsset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	LoadConsoleWeb(router)

	indexResp := requestConsole(router, ConsoleBasePath)
	assetPath := findFirstAssetPath(indexResp.Body.String())
	if assetPath == "" {
		t.Fatalf("index did not contain a /jmateconsole/assets script")
	}

	assetResp := requestConsole(router, assetPath)
	if assetResp.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", assetPath, assetResp.Code)
	}
	if strings.Contains(assetResp.Body.String(), `<div id="root"></div>`) {
		t.Fatalf("GET %s returned index fallback instead of asset", assetPath)
	}
}

func TestLoadConsoleWebDoesNotHandleOtherPrefixes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	LoadConsoleWeb(router)

	for _, path := range []string{"/", "/jmate/user/login", "/botmsgs/msgcallback"} {
		resp := requestConsole(router, path)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("GET %s status = %d, want 404", path, resp.Code)
		}
	}
}

func requestConsole(router *gin.Engine, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func findFirstAssetPath(indexHTML string) string {
	marker := `src="/jmateconsole/assets/`
	start := strings.Index(indexHTML, marker)
	if start < 0 {
		return ""
	}
	start += len(`src="`)
	end := strings.Index(indexHTML[start:], `"`)
	if end < 0 {
		return ""
	}
	return indexHTML[start : start+end]
}
