package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAllowedHostsPersistence(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	first, err := Open(dataDir, 30)
	if err != nil {
		t.Fatal(err)
	}
	hosts, found, err := first.LoadAllowedHosts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if found || len(hosts) != 0 {
		t.Fatalf("initial setting = %#v, found=%v", hosts, found)
	}
	want := []string{"api.openai.com", "api.anthropic.com"}
	if err := first.SaveAllowedHosts(ctx, want); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Open(dataDir, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	got, found, err := second.LoadAllowedHosts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !found || !reflect.DeepEqual(got, want) {
		t.Fatalf("setting = %#v, found=%v, want %#v", got, found, want)
	}
}

func TestOpenMigratesEventDetails(t *testing.T) {
	dataDir := t.TempDir()
	initial, err := Open(dataDir, 30)
	if err != nil {
		t.Fatal(err)
	}
	legacy := Event{
		RequestID: "legacy-event", Method: "POST", Protocol: "generic",
		UpstreamScheme: "https", UpstreamHost: "legacy.example.com", UpstreamPath: "/v1",
		Flags: "E", Status: 200, RuleHits: map[string]int{"EMAIL": 2}, RedactionCount: 2,
	}
	if err := initial.InsertEvent(context.Background(), legacy); err != nil {
		t.Fatal(err)
	}
	if err := initial.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", filepath.Join(dataDir, "gateway.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"redaction_fields_json", "restore_unique_count", "restore_unresolved_count", "restore_degraded_count", "restore_status"} {
		if _, err := db.Exec("ALTER TABLE events DROP COLUMN " + column); err != nil {
			db.Close()
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	migrated, err := Open(dataDir, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()
	event := Event{
		RequestID: "migration-test", Method: "POST", Protocol: "generic",
		UpstreamScheme: "https", UpstreamHost: "example.com", UpstreamPath: "/v1",
		Flags: "E", Status: 200, RedactionFields: []string{"$.message"}, RuleHits: map[string]int{"EMAIL": 1},
		RestoreCount: 3, RestoreUniqueCount: 1, RestoreUnresolvedCount: 2, RestoreDegradedCount: 1, RestoreStatus: "partial",
	}
	if err := migrated.InsertEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	events, err := migrated.Events(context.Background(), 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || !reflect.DeepEqual(events[0].RedactionFields, []string{"$.message"}) {
		t.Fatalf("migrated event = %#v", events)
	}
	old, err := migrated.Events(context.Background(), 10, "legacy.example.com")
	if err != nil || len(old) != 1 {
		t.Fatalf("legacy events = %#v, err=%v", old, err)
	}
	if old[0].RequestID != legacy.RequestID || old[0].RuleHits["EMAIL"] != 2 || old[0].RedactionCount != 2 || old[0].RedactionFields == nil || len(old[0].RedactionFields) != 0 {
		t.Fatalf("legacy details changed: %#v", old[0])
	}
	if old[0].RestoreStatus != "unavailable" || old[0].RestoreUniqueCount != 0 {
		t.Fatalf("historical diagnostics fabricated: %#v", old[0])
	}
	if err := migrated.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(dataDir, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	events, err = reopened.Events(context.Background(), 10, "example.com")
	if err != nil || len(events) != 1 || !reflect.DeepEqual(events[0].RedactionFields, event.RedactionFields) || !reflect.DeepEqual(events[0].RuleHits, event.RuleHits) {
		t.Fatalf("persisted details = %#v, err=%v", events, err)
	}
	if events[0].RestoreCount != 3 || events[0].RestoreUniqueCount != 1 || events[0].RestoreUnresolvedCount != 2 || events[0].RestoreDegradedCount != 1 || events[0].RestoreStatus != "partial" {
		t.Fatalf("restoration diagnostics not persisted: %#v", events[0])
	}
}

func TestQueryEventsPaginationAndSearch(t *testing.T) {
	eventStore, err := Open(t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer eventStore.Close()
	ctx := context.Background()
	for index := 1; index <= 205; index++ {
		event := Event{
			RequestID: fmt.Sprintf("request-%d", index), Timestamp: "2026-09-13T12:00:00Z",
			UpstreamHost: "api.example.com", UpstreamPath: fmt.Sprintf("/v1/%d", index),
			Protocol: "generic", Flags: "E", Status: 200,
		}
		if index%2 == 0 {
			event.UpstreamHost = "other.example.com"
		}
		if index == 1 {
			event.UpstreamPath = "/archive_100%/old"
			event.Protocol = "openai_chat"
			event.Flags = "HP"
			event.Status = 403
		}
		if err := eventStore.InsertEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}

	seen := make(map[int64]bool)
	for page := 1; page <= 5; page++ {
		result, err := eventStore.QueryEvents(ctx, EventFilter{Page: page, Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		wantLength := min(50, 205-(page-1)*50)
		if result.Total != 205 || result.Page != page || result.Limit != 50 || len(result.Events) != wantLength {
			t.Fatalf("page %d: total=%d page=%d limit=%d length=%d", page, result.Total, result.Page, result.Limit, len(result.Events))
		}
		for index, event := range result.Events {
			wantID := int64(205 - (page-1)*50 - index)
			if event.ID != wantID || seen[event.ID] {
				t.Fatalf("page %d: event id=%d, want=%d, duplicate=%v", page, event.ID, wantID, seen[event.ID])
			}
			seen[event.ID] = true
		}
	}
	if len(seen) != 205 {
		t.Fatalf("only read %d events", len(seen))
	}

	tests := []struct {
		name                  string
		filter                EventFilter
		total, page, limit, n int
		firstID               int64
	}{
		{"defaults", EventFilter{}, 205, 1, 100, 100, 205},
		{"invalid limit and page", EventFilter{Limit: 501, Page: -1}, 205, 1, 100, 100, 205},
		{"maximum limit", EventFilter{Limit: 500}, 205, 1, 500, 205, 205},
		{"clamp last page", EventFilter{Limit: 50, Page: int(^uint(0) >> 1)}, 205, 5, 50, 5, 5},
		{"host filter", EventFilter{Limit: 100, Page: 2, UpstreamHost: " other.example.com "}, 102, 2, 100, 2, 4},
		{"search before pagination", EventFilter{Limit: 20, Page: 2, Search: " API.EXAMPLE.COM "}, 103, 2, 20, 20, 165},
		{"historical path", EventFilter{Search: "archive"}, 1, 1, 100, 1, 1},
		{"protocol", EventFilter{Search: "OPENAI_CHAT"}, 1, 1, 100, 1, 1},
		{"flags", EventFilter{Search: "hp"}, 1, 1, 100, 1, 1},
		{"status", EventFilter{Search: "403"}, 1, 1, 100, 1, 1},
		{"literal wildcard characters", EventFilter{Search: "_100%"}, 1, 1, 100, 1, 1},
		{"combined filters", EventFilter{UpstreamHost: "other.example.com", Search: "archive", Page: 3}, 0, 1, 100, 0, 0},
		{"parameterized search", EventFilter{Search: "' OR 1=1 --"}, 0, 1, 100, 0, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := eventStore.QueryEvents(ctx, test.filter)
			if err != nil {
				t.Fatal(err)
			}
			if result.Total != test.total || result.Page != test.page || result.Limit != test.limit || result.Events == nil || len(result.Events) != test.n {
				t.Fatalf("total=%d page=%d limit=%d length=%d; want total=%d page=%d limit=%d length=%d", result.Total, result.Page, result.Limit, len(result.Events), test.total, test.page, test.limit, test.n)
			}
			if test.n > 0 && result.Events[0].ID != test.firstID {
				t.Fatalf("first event id=%d, want %d", result.Events[0].ID, test.firstID)
			}
		})
	}
}
