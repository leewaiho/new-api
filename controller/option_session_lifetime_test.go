package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/auth_setting"
	"github.com/gin-gonic/gin"
)

func TestUpdateOptionRejectsInvalidDashboardSessionLifetime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, value := range []string{"0", "-1", "3651", "forever"} {
		t.Run(value, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPut, "/api/option", bytes.NewBufferString(`{"key":"`+auth_setting.OptionDashboardSessionLifetimeDays+`","value":"`+value+`"}`))
			ctx.Request.Header.Set("Content-Type", "application/json")

			UpdateOption(ctx)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			var response struct {
				Success bool `json:"success"`
			}
			if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Success {
				t.Fatal("invalid dashboard session lifetime returned success")
			}
		})
	}
}
