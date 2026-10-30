package middleware_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fastenhealth/fasten-onprem/backend/pkg"
	"github.com/fastenhealth/fasten-onprem/backend/pkg/web/middleware"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestLoggerMiddlewareDoesNotRecordBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, level := range []logrus.Level{logrus.InfoLevel, logrus.DebugLevel, logrus.TraceLevel} {
		t.Run(level.String(), func(t *testing.T) {
			for _, status := range []int{http.StatusCreated, http.StatusBadRequest, http.StatusInternalServerError} {
				t.Run(http.StatusText(status), func(t *testing.T) {
					var output bytes.Buffer
					logger := logrus.New()
					logger.SetOutput(&output)
					logger.SetFormatter(&logrus.JSONFormatter{})
					logger.SetLevel(level)
					entry := logrus.NewEntry(logger)
					requestBody := `{"password":"synthetic-private-request"}`
					responseBody := `{"resourceType":"Patient","id":"synthetic-private-response"}`
					router := gin.New()
					router.Use(middleware.LoggerMiddleware(entry))
					router.POST("/api/synthetic", func(c *gin.Context) {
						require.Same(t, entry, c.MustGet(pkg.ContextKeyTypeLogger))
						body, err := io.ReadAll(c.Request.Body)
						require.NoError(t, err)
						require.Equal(t, requestBody, string(body))
						c.Data(status, "application/json", []byte(responseBody))
					})
					request := httptest.NewRequest(http.MethodPost, "/api/synthetic", strings.NewReader(requestBody))
					response := httptest.NewRecorder()
					router.ServeHTTP(response, request)
					require.Equal(t, status, response.Code)
					require.Equal(t, responseBody, response.Body.String())
					require.NotContains(t, output.String(), "synthetic-private-request")
					require.NotContains(t, output.String(), "synthetic-private-response")
					var record map[string]interface{}
					decoder := json.NewDecoder(&output)
					require.NoError(t, decoder.Decode(&record))
					require.Equal(t, float64(status), record["statusCode"])
					require.Equal(t, float64(len(responseBody)), record["respLength"])
					require.Equal(t, http.MethodPost, record["method"])
					require.Equal(t, "/api/synthetic", record["path"])
					require.ErrorIs(t, decoder.Decode(&record), io.EOF)
				})
			}
		})
	}
}

func TestLoggerMiddlewareLeavesUnreadBodyForHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	for _, level := range []logrus.Level{logrus.InfoLevel, logrus.DebugLevel} {
		t.Run(level.String(), func(t *testing.T) {
			logger.SetLevel(level)
			body := &observedRequestBody{Reader: strings.NewReader("synthetic-stream")}
			router := gin.New()
			router.Use(middleware.LoggerMiddleware(logrus.NewEntry(logger)))
			router.POST("/api/synthetic", func(c *gin.Context) {
				require.Zero(t, body.reads, "logging must not read a request body before its handler")
				require.Same(t, body, c.Request.Body, "logging must preserve the original body and closer")
				c.Status(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodPost, "/api/synthetic", body)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, http.StatusNoContent, response.Code)
			require.Zero(t, body.reads)
		})
	}
}

type observedRequestBody struct {
	io.Reader
	reads int
}

func (r *observedRequestBody) Read(p []byte) (int, error) {
	r.reads++
	return r.Reader.Read(p)
}

func (r *observedRequestBody) Close() error { return nil }
