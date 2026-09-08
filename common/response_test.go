package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestJSONUnauthorizedUsesHTTP401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/", func(c *gin.Context) { ErrUnAuthorized(c, "invalid authorization") })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
