package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSaveSharedPoolCustomPriceRequiresAuthenticatedOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/biz/pools/7/pricing", strings.NewReader(`{"model_name":"owner-model"}`))
	c.Params = gin.Params{{Key: "id", Value: "7"}}

	(&BizDecipherHandler{}).SaveSharedPoolCustomPrice(c)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestListSharedPoolModelPricingRejectsInvalidPoolID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/biz/pools/not-a-number/pricing", nil)
	c.Params = gin.Params{{Key: "id", Value: "not-a-number"}}

	(&BizDecipherHandler{}).ListSharedPoolModelPricing(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
