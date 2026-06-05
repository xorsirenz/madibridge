package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type LoggingRoundTripper struct {
	Next            http.RoundTripper
	Logger          *slog.Logger
	MaxBodySize     int
	LogRequests     bool
	LogResponses    bool
	PrettyPrintJSON bool
}

func (rt *LoggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	logger := rt.logger()
	start := time.Now()

	if rt.LogRequests {
		rt.logRequest(req, logger)
	}

	resp, err := rt.transport().RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		logger.Error(
			"http request failed",
			slog.String("method", req.Method),
			slog.String("url", req.URL.String()),
			slog.Duration("duration", duration),
			slog.Any("error", err),
		)

		return nil, err
	}

	if rt.LogResponses {
		rt.logResponse(req.Context(), req, resp, duration, logger)
	}

	return resp, nil
}

func (rt *LoggingRoundTripper) transport() http.RoundTripper {
	if rt.Next != nil {
		return rt.Next
	}

	return http.DefaultTransport
}

func (rt *LoggingRoundTripper) logger() *slog.Logger {
	if rt.Logger != nil {
		return rt.Logger
	}

	return slog.Default()
}

func (rt *LoggingRoundTripper) logRequest(
	req *http.Request,
	logger *slog.Logger,
) {
	headers := make(map[string]string)

	for k, v := range req.Header {
		if strings.EqualFold(k, "Authorization") {
			headers[k] = "[REDACTED]"
			continue
		}

		headers[k] = strings.Join(v, ", ")
	}

	logger.Debug(
		"http request",
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.Any("headers", headers),
	)
}

func (rt *LoggingRoundTripper) logResponse(
	ctx context.Context,
	req *http.Request,
	resp *http.Response,
	duration time.Duration,
	logger *slog.Logger,
) {
	if resp.Body == nil {
		logger.DebugContext(
			ctx,
			"http response",
			slog.String("method", req.Method),
			slog.String("url", req.URL.String()),
			slog.Int("status", resp.StatusCode),
			slog.Duration("duration", duration),
			slog.String("body", "<nil>"),
		)

		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error(
			"failed to read response body",
			slog.String("method", req.Method),
			slog.String("url", req.URL.String()),
			slog.Any("error", err),
		)

		return
	}

	_ = resp.Body.Close()

	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))

	maxSize := rt.MaxBodySize
	if maxSize <= 0 {
		maxSize = 16 * 1024
	}

	loggedBody := body

	if len(loggedBody) > maxSize {
		loggedBody = loggedBody[:maxSize]
	}

	bodyString := rt.formatBody(
		resp.Header.Get("Content-Type"),
		loggedBody,
	)

	logger.DebugContext(
		ctx,
		"matrix response",
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.Int("status", resp.StatusCode),
		slog.Duration("duration", duration),
	)
	fmt.Println("\n========== MATRIX RESPONSE ==========")
	fmt.Println("METHOD:", req.Method)
	fmt.Println("URL:", req.URL.String())
	fmt.Println("STATUS:", resp.Status)
	fmt.Println("DURATION:", duration)
	fmt.Println()
	fmt.Println(bodyString)
	fmt.Println("=====================================")
}

func (rt *LoggingRoundTripper) formatBody(
	contentType string,
	body []byte,
) string {
	if !rt.PrettyPrintJSON {
		return string(body)
	}

	if !strings.Contains(strings.ToLower(contentType), "json") {
		return string(body)
	}

	var pretty bytes.Buffer

	if err := json.Indent(&pretty, body, "", "  "); err != nil {
		return string(body)
	}

	return pretty.String()
}
