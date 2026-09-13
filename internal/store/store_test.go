package store

import (
	"context"
	"database/sql"
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

func TestOpenMigratesRedactionFieldsColumn(t *testing.T) {
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
	if _, err := db.Exec(`ALTER TABLE events DROP COLUMN redaction_fields_json`); err != nil {
		db.Close()
		t.Fatal(err)
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
}
