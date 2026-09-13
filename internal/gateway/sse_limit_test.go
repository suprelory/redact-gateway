package gateway

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/suprelory/redact-gateway/internal/config"
	"github.com/suprelory/redact-gateway/internal/store"
)

func TestProxyStreamLimitKeepsCompletedOutputAndRecordsFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"accepted\"}\n\n")
		fmt.Fprintf(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"%s\"}\n\n", strings.Repeat("x", 2048))
	}))
	defer upstream.Close()
	eventStore, err := store.Open(t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer eventStore.Close()
	proxy := NewProxy(config.Config{AllowPrivateHosts: true, MaxBodyBytes: 256, MaxRedactions: 10}, eventStore, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	request := httptest.NewRequest(http.MethodPost, "http://gateway/E$"+upstream.URL+"/v1/responses", strings.NewReader(`{"input":"hello"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, request)
	events, err := eventStore.Events(context.Background(), 10, "")
	if err != nil || len(events) != 1 {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	event := events[0]
	if recorder.Code != 200 || !strings.Contains(recorder.Body.String(), "accepted") || strings.Contains(recorder.Body.String(), strings.Repeat("x", 100)) || event.ErrorClass != "stream_limit_exceeded" || event.RestoreStatus != "response_error" || event.ResponseBytes != int64(recorder.Body.Len()) {
		t.Fatalf("stream limit lost output or diagnostics: %s / %+v", recorder.Body.String(), event)
	}
}

func TestResponseBodyLimitCannotOverflow(t *testing.T) {
	maximum := int64(^uint(0)>>1) - 1
	for _, test := range []struct{ input, want int64 }{{1024, 2048}, {maximum, maximum}, {0, 32 * 1024 * 1024}} {
		proxy := &Proxy{cfg: config.Config{MaxBodyBytes: test.input}}
		if got := proxy.responseBodyLimit(); got != test.want || got+1 <= 0 {
			t.Fatalf("input %d: limit %d, want %d", test.input, got, test.want)
		}
	}
}
