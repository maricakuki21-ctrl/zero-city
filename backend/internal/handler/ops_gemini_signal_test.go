package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpsGeminiPolicySignal_RequestScopePersistence(t *testing.T) {
	for _, nonStream := range []bool{false, true} {
		for _, priorFailure := range []bool{false, true} {
			for _, skipPrior := range []bool{false, true} {
				t.Run(fmt.Sprintf("nonstream=%t/prior=%t/skip=%t", nonStream, priorFailure, skipPrior), func(t *testing.T) {
					setupOpsErrorLogTestQueue(t, 4)
					ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
					router := gin.New()
					router.Use(OpsErrorLoggerMiddleware(ops))
					const body = `{"candidates":[{"finishReason":"SAFETY"}]}`
					router.POST("/v1beta/models/test:generateContent", func(c *gin.Context) {
						c.Set(opsAccountIDKey, int64(123))
						c.Set(opsStreamKey, !nonStream)
						if priorFailure {
							c.Set(service.OpsUpstreamStatusCodeKey, http.StatusServiceUnavailable)
							c.Set(service.OpsUpstreamErrorMessageKey, "prior account unavailable")
							c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{{
								AccountID: 99, UpstreamStatusCode: 503, Message: "prior account unavailable",
							}})
						}
						c.Set(service.OpsSkipPassthroughKey, skipPrior)
						service.MarkOpsStreamErrorValue(c, service.OpsStreamError{
							ErrType: "invalid_request_error", Code: "SAFETY", Message: "Gemini content policy stop",
							IntendedStatus: 400, RequestScoped: true, NonStream: nonStream,
							CountTowardsSLA: true, // Request scope must override even an inconsistent caller.
						})
						c.Data(http.StatusOK, "application/json", []byte(body))
					})
					recorder := httptest.NewRecorder()
					router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1beta/models/test:generateContent", nil))
					require.Equal(t, http.StatusOK, recorder.Code)
					require.Equal(t, body, recorder.Body.String())
					expected := int64(1)
					if priorFailure && !skipPrior {
						expected++
					}
					require.Equal(t, expected, OpsErrorLogQueueLength())
					job := <-opsErrorLogQueue
					require.Equal(t, http.StatusBadRequest, job.entry.StatusCode)
					require.True(t, job.entry.IsBusinessLimited)
					require.Equal(t, "request", job.entry.ErrorPhase)
					require.Equal(t, "client", job.entry.ErrorOwner)
					require.Equal(t, "client_request", job.entry.ErrorSource)
					require.Equal(t, !nonStream, job.entry.Stream)
					require.Nil(t, job.entry.AccountID)
					require.Nil(t, job.entry.UpstreamStatusCode)
					require.Empty(t, job.entry.UpstreamErrors)
					require.Contains(t, job.entry.ErrorBody, "SAFETY")
					if expected == 2 {
						recovered := <-opsErrorLogQueue
						require.Equal(t, http.StatusOK, recovered.entry.StatusCode)
						require.Equal(t, int64(99), *recovered.entry.AccountID)
						require.Equal(t, "provider", recovered.entry.ErrorOwner)
						require.NotNil(t, recovered.entry.UpstreamErrorsJSON)
						events, err := service.ParseOpsUpstreamErrors(*recovered.entry.UpstreamErrorsJSON)
						require.NoError(t, err)
						require.Len(t, events, 1)
						require.Equal(t, int64(99), events[0].AccountID)
						require.Equal(t, 503, events[0].UpstreamStatusCode)
					}
				})
			}
		}
	}
}

func TestOpsGeminiErrorSignal_IsFailureNotRecoveredAttempt(t *testing.T) {
	for _, nonStream := range []bool{false, true} {
		t.Run(fmt.Sprintf("nonstream=%t", nonStream), func(t *testing.T) {
			setupOpsErrorLogTestQueue(t, 4)
			ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			router := gin.New()
			router.Use(OpsErrorLoggerMiddleware(ops))
			router.GET("/v1/messages", func(c *gin.Context) {
				c.Set(opsAccountIDKey, int64(123))
				c.Set(service.OpsUpstreamStatusCodeKey, 503)
				c.Set(service.OpsUpstreamErrorMessageKey, "Gemini unavailable")
				service.MarkOpsStreamErrorValue(c, service.OpsStreamError{
					ErrType: "upstream_error", Code: "UNAVAILABLE", Message: "Gemini unavailable",
					IntendedStatus: 503, CountTowardsSLA: true, NonStream: nonStream,
				})
				c.Data(http.StatusOK, "application/json", []byte(`{"error":{"code":503}}`))
			})
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/messages", nil))
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, int64(1), OpsErrorLogQueueLength())
			job := <-opsErrorLogQueue
			require.Equal(t, 503, job.entry.StatusCode)
			require.Equal(t, 503, *job.entry.UpstreamStatusCode)
			require.False(t, job.entry.IsBusinessLimited)
			require.Equal(t, int64(123), *job.entry.AccountID)
			require.Equal(t, !nonStream, job.entry.Stream)
			require.Equal(t, "provider", job.entry.ErrorOwner)
		})
	}
}
