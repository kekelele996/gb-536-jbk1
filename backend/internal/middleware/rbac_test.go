package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"corpus-annotation-agreement-control/backend/internal/constants"
	"corpus-annotation-agreement-control/backend/internal/dto"
)

func TestRBAC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		actor      *dto.Actor
		wantStatus int
	}{
		{"allowed adjudicator", &dto.Actor{ID: 1, Role: constants.RoleAdjudicator}, http.StatusNoContent},
		{"forbidden annotator", &dto.Actor{ID: 2, Role: constants.RoleAnnotator}, http.StatusForbidden},
		{"missing actor", nil, http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine := gin.New()
			engine.GET("/review", func(context *gin.Context) {
				if test.actor != nil {
					context.Set(actorContextKey, *test.actor)
				}
			}, RBAC(constants.RoleAdjudicator, constants.RoleAdmin), func(context *gin.Context) { context.Status(http.StatusNoContent) })
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/review", nil))
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}
