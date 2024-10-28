package exporter

import (
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/qjs/quanti-tea/server/db"
)

type Exporter struct {
	DB      *db.Database
	Metrics *prometheus.GaugeVec
}

func NewExporter(database *db.Database) *Exporter {
	metrics := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dynamic_metrics",
			Help: "Dynamically added metrics",
		},
		[]string{"metric_name", "type", "unit", "reset_daily"},
	)

	prometheus.MustRegister(metrics)

	return &Exporter{
		DB:      database,
		Metrics: metrics,
	}
}

func (e *Exporter) UpdateMetrics() {
	metrics, err := e.DB.GetMetrics()
	if err != nil {
		log.Printf("Error fetching metrics from DB: %v", err)
		return
	}

	// Reset existing metrics to avoid stale data
	e.Metrics.Reset()

	for _, m := range metrics {
		e.Metrics.With(prometheus.Labels{
			"metric_name": m.MetricName,
			"type":        m.Type,
			"unit":        m.Unit,
			"reset_daily": boolToString(m.ResetDaily),
		}).Set(float64(m.Value))
	}
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func (e *Exporter) Start(addr string) {
	// Periodically update metrics
	go func() {
		for {
			e.UpdateMetrics()
			// Update interval can be adjusted as needed
			// For example, every 10 seconds
			time.Sleep(10 * time.Second)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Starting Prometheus exporter at %s/metrics", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start Prometheus exporter: %v", err)
	}
}
