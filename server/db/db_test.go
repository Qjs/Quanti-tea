package db

import (
	"path/filepath"
	"testing"
)

func newTestDB(t *testing.T) *Database {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	database, err := NewDatabase(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	return database
}

func TestAddAndGetMetric(t *testing.T) {
	db := newTestDB(t)
	metric := DBMetric{
		MetricName: "steps",
		Type:       "health",
		Unit:       "count",
		Value:      10,
		ResetDaily: false,
	}
	if err := db.AddMetric(metric); err != nil {
		t.Fatalf("AddMetric failed: %v", err)
	}

	got, err := db.GetMetric("steps")
	if err != nil {
		t.Fatalf("GetMetric failed: %v", err)
	}
	if got.MetricName != metric.MetricName || got.Type != metric.Type || got.Unit != metric.Unit || got.Value != metric.Value || got.ResetDaily != metric.ResetDaily {
		t.Errorf("retrieved metric mismatch: %+v vs %+v", got, metric)
	}
}

func TestUpdateMetric(t *testing.T) {
	db := newTestDB(t)
	m := DBMetric{MetricName: "weight", Type: "health", Unit: "kg", Value: 70}
	if err := db.AddMetric(m); err != nil {
		t.Fatalf("AddMetric failed: %v", err)
	}
	if err := db.UpdateMetric("weight", 72); err != nil {
		t.Fatalf("UpdateMetric failed: %v", err)
	}
	got, err := db.GetMetric("weight")
	if err != nil {
		t.Fatalf("GetMetric failed: %v", err)
	}
	if got.Value != 72 {
		t.Errorf("expected value 72 got %f", got.Value)
	}
}

func TestIncrementDecrementMetric(t *testing.T) {
	db := newTestDB(t)
	m := DBMetric{MetricName: "pushups", Type: "health", Unit: "count", Value: 5}
	if err := db.AddMetric(m); err != nil {
		t.Fatalf("AddMetric failed: %v", err)
	}
	if err := db.IncrementMetric("pushups", 3); err != nil {
		t.Fatalf("IncrementMetric failed: %v", err)
	}
	got, _ := db.GetMetric("pushups")
	if got.Value != 8 {
		t.Errorf("expected 8 got %f", got.Value)
	}
	if err := db.DecrementMetric("pushups", 2); err != nil {
		t.Fatalf("DecrementMetric failed: %v", err)
	}
	got, _ = db.GetMetric("pushups")
	if got.Value != 6 {
		t.Errorf("expected 6 got %f", got.Value)
	}
	if err := db.DecrementMetric("pushups", 10); err == nil {
		t.Fatalf("expected error when decrementing below zero")
	}
}

func TestDeleteMetric(t *testing.T) {
	db := newTestDB(t)
	m := DBMetric{MetricName: "calories", Type: "health", Unit: "kcal"}
	if err := db.AddMetric(m); err != nil {
		t.Fatalf("AddMetric failed: %v", err)
	}
	if err := db.DeleteMetric("calories"); err != nil {
		t.Fatalf("DeleteMetric failed: %v", err)
	}
	if _, err := db.GetMetric("calories"); err == nil {
		t.Fatalf("expected error retrieving deleted metric")
	}
}

func TestResetDailyMetrics(t *testing.T) {
	db := newTestDB(t)
	daily := DBMetric{MetricName: "water", Type: "health", Unit: "ml", Value: 500, ResetDaily: true}
	regular := DBMetric{MetricName: "books", Type: "brain", Unit: "count", Value: 2, ResetDaily: false}
	if err := db.AddMetric(daily); err != nil {
		t.Fatalf("AddMetric daily failed: %v", err)
	}
	if err := db.AddMetric(regular); err != nil {
		t.Fatalf("AddMetric regular failed: %v", err)
	}
	if err := db.ResetDailyMetrics(); err != nil {
		t.Fatalf("ResetDailyMetrics failed: %v", err)
	}
	gotDaily, _ := db.GetMetric("water")
	if gotDaily.Value != 0 {
		t.Errorf("daily metric not reset: %f", gotDaily.Value)
	}
	gotReg, _ := db.GetMetric("books")
	if gotReg.Value != 2 {
		t.Errorf("regular metric altered: %f", gotReg.Value)
	}
}
