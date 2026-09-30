package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRatioRounds(t *testing.T) {
	b, err := json.Marshal(struct{ V Ratio }{Ratio(1.0 / 3.0)})
	if err != nil || string(b) != `{"V":0.3333}` {
		t.Errorf("marshal = %s err=%v", b, err)
	}
}

func TestEntryRoundTripAndErrors(t *testing.T) {
	c := Commit{SHA: "abc", Date: "2026-01-01T00:00:00Z", Subject: "s"}
	ok, err := json.Marshal(Entry{Commit: c, Snapshot: &Snapshot{SchemaVersion: "1", Module: "m"}})
	if err != nil {
		t.Fatal(err)
	}
	var e Entry
	if err := json.Unmarshal(ok, &e); err != nil || e.Snapshot == nil || e.Snapshot.Module != "m" {
		t.Fatalf("snapshot entry: %+v err=%v", e, err)
	}
	failed, _ := json.Marshal(Entry{Commit: c, Error: "boom"})
	if !strings.Contains(string(failed), `"error":"boom"`) || strings.Contains(string(failed), "packages") {
		t.Errorf("failed entry = %s", failed)
	}
	if err := json.Unmarshal(failed, &e); err != nil || e.Error != "boom" || e.Snapshot != nil || e.Commit.SHA != "abc" {
		t.Errorf("failed entry decode: %+v err=%v", e, err)
	}
	if err := json.Unmarshal([]byte(`[1]`), &e); err == nil {
		t.Error("expected error for a non-object entry")
	}
	if err := json.Unmarshal([]byte(`{"commit":{"sha":"x"},"packages":"bad"}`), &e); err == nil {
		t.Error("expected error for a malformed snapshot")
	}
	blank, _ := json.Marshal(Entry{})
	if !strings.Contains(string(blank), `"error"`) {
		t.Errorf("entry without snapshot must marshal as a failed entry: %s", blank)
	}
}
func TestVolatilityFieldsAreOmittedWhenZero(t *testing.T) {
	b, err := json.Marshal(struct {
		P Package
		C Config
	}{})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"churn", "volatility"} {
		if strings.Contains(string(b), key) {
			t.Errorf("%q must be omitted when zero: %s", key, b)
		}
	}
	b, _ = json.Marshal(Package{Churn: 3, Volatility: 0.5})
	if !strings.Contains(string(b), `"churn":3`) || !strings.Contains(string(b), `"volatility":0.5`) {
		t.Errorf("fields missing: %s", b)
	}
}
