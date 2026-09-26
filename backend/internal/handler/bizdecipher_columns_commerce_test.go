package handler

import (
	"net/http"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestColumnCommerceAdministrativeGuard(t *testing.T) {
	h := &BizDecipherHandler{}
	for name, action := range map[string]func(*gin.Context){
		"policy": h.AdminSetColumnCommercePolicy,
		"refund": h.AdminRefundColumnPurchase,
	} {
		t.Run(name, func(t *testing.T) {
			w, c := newPollHandlerContext(http.MethodPost, "/?admin=true", `{}`, "1", 0)
			action(c)
			require.Equal(t, http.StatusUnauthorized, w.Code)
			w, c = newPollHandlerContext(http.MethodPost, "/?admin=true", `{"actor_id":1}`, "1", 7)
			c.Set(string(middleware2.ContextKeyUserRole), "user")
			action(c)
			require.Equal(t, http.StatusForbidden, w.Code)
		})
	}
	w, c := newPollHandlerContext(http.MethodPost, "/", `{}`, "1", 0)
	h.PurchaseColumn(c)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}
