package store

import (
	"context"
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
