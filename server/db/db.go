// db.go
package db

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	_ "modernc.org/sqlite" // SQLite driver
)

// Database encapsulates the SQLite connection and a mutex for thread safety
type Database struct {
	conn *sql.DB
	mu   sync.RWMutex
}

// DBMetric represents a metric stored in the database
type DBMetric struct {
	MetricName string
	Type       string
	Unit       string
	Value      float64
	ResetDaily bool
	LastReset  time.Time
}

// NewDatabase initializes a new Database instance
func NewDatabase(dbPath string) (*Database, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	db := &Database{conn: conn}

	if err := db.init(); err != nil {
		return nil, err
	}

	return db, nil
}

// init creates the metrics table if it doesn't exist
func (db *Database) init() error {
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS metrics (
		metric_name TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		unit TEXT NOT NULL,
		value DOUBLE NOT NULL DEFAULT 0,
		reset_daily BOOLEAN NOT NULL DEFAULT FALSE,
		last_reset TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := db.conn.Exec(createTableQuery)
	return err
}

// AddMetric inserts a new metric into the database
func (db *Database) AddMetric(metric DBMetric) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	insertQuery := `INSERT INTO metrics (metric_name, type, unit, value, reset_daily, last_reset) VALUES (?, ?, ?, ?, ?, ?);`

	_, err := db.conn.Exec(insertQuery, metric.MetricName, metric.Type, metric.Unit, metric.Value, metric.ResetDaily, metric.LastReset)
	if err != nil {
		return fmt.Errorf("failed to add metric: %w", err)
	}

	return nil
}

// DeleteMetric removes a metric from the database by its name.
func (db *Database) DeleteMetric(metricName string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	deleteQuery := `DELETE FROM metrics WHERE metric_name = ?;`

	result, err := db.conn.Exec(deleteQuery, metricName)
	if err != nil {
		return fmt.Errorf("failed to delete metric: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected during delete: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("metric '%s' does not exist", metricName)
	}

	return nil
}

// UpdateMetric sets the value of a metric to a new specified value
func (db *Database) UpdateMetric(metricName string, newValue float64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Update the metric's value and optionally update the last_reset time
	updateQuery := `UPDATE metrics SET value = ?, last_reset = ? WHERE metric_name = ?;`

	now := time.Now()

	result, err := db.conn.Exec(updateQuery, newValue, now, metricName)
	if err != nil {
		return fmt.Errorf("failed to update metric: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected during update: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("metric %s does not exist", metricName)
	}

	return nil
}

// GetMetrics retrieves all metrics from the database
func (db *Database) GetMetrics() ([]DBMetric, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `SELECT metric_name, type, unit, value, reset_daily, last_reset FROM metrics;`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}
	defer rows.Close()

	var metrics []DBMetric
	for rows.Next() {
		var m DBMetric
		var lastResetStr string
		if err := rows.Scan(&m.MetricName, &m.Type, &m.Unit, &m.Value, &m.ResetDaily, &lastResetStr); err != nil {
			return nil, fmt.Errorf("failed to scan metric: %w", err)
		}
		m.LastReset, err = time.Parse("2006-01-02 15:04:05", lastResetStr)
		if err != nil {
			// If parsing fails, default to current time
			m.LastReset = time.Now()
		}
		metrics = append(metrics, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return metrics, nil
}

// GetMetric retrieves a single metric by its name
func (db *Database) GetMetric(metricName string) (*DBMetric, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `SELECT metric_name, type, unit, value, reset_daily, last_reset FROM metrics WHERE metric_name = ?;`
	row := db.conn.QueryRow(query, metricName)

	var m DBMetric
	var lastResetStr string
	if err := row.Scan(&m.MetricName, &m.Type, &m.Unit, &m.Value, &m.ResetDaily, &lastResetStr); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("metric %s does not exist", metricName)
		}
		return nil, fmt.Errorf("failed to scan metric: %w", err)
	}
	var err error
	m.LastReset, err = time.Parse("2006-01-02 15:04:05", lastResetStr)
	if err != nil {
		m.LastReset = time.Now()
	}

	return &m, nil
}

// IncrementMetric increases the value of a metric by a specified amount
func (db *Database) IncrementMetric(metricName string, increment float64) error {

	// Retrieve the current metric
	metric, err := db.GetMetric(metricName)
	if err != nil {
		return fmt.Errorf("increment failed: %w", err)
	}

	// Calculate the new value
	newValue := metric.Value + increment

	// Update the metric with the new value
	err = db.UpdateMetric(metricName, newValue)
	if err != nil {
		return fmt.Errorf("failed to update metric after incrementing: %w", err)
	}

	return nil
}

// DecrementMetric decreases the value of a metric by a specified amount
func (db *Database) DecrementMetric(metricName string, decrement float64) error {

	// Retrieve the current metric
	metric, err := db.GetMetric(metricName)
	if err != nil {
		return fmt.Errorf("decrement failed: %w", err)
	}

	// Calculate the new value
	newValue := metric.Value - decrement

	// Ensure that the new value does not go below zero
	if newValue < 0 {
		return fmt.Errorf("decrement failed: metric %s value cannot be negative", metricName)
	}

	// Update the metric with the new value
	err = db.UpdateMetric(metricName, newValue)
	if err != nil {
		return fmt.Errorf("failed to update metric after decrementing: %v", err)
	}

	return nil
}

// ResetDailyMetrics resets all metrics that are marked to reset daily and haven't been reset today
func (db *Database) ResetDailyMetrics() error {

	// Get all metrics
	metrics, err := db.GetMetrics()
	if err != nil {
		return fmt.Errorf("failed to retrieve metrics: %w", err)
	}

	// Iterate over the metrics and reset those that are marked to reset daily
	for _, metric := range metrics {
		if metric.ResetDaily {
			// Reset the metric's value to 0 and update the last reset time
			err := db.UpdateMetric(metric.MetricName, 0)
			if err != nil {
				log.Printf("Failed to reset metric %s: %v", metric.MetricName, err)
			} else {
				log.Printf("Reset metric %s to 0", metric.MetricName)
			}
		}
	}

	return nil
}

// StartDailyResetScheduler starts a scheduler that resets daily metrics at midnight using Go channels.
func (db *Database) StartDailyResetScheduler(stopChan chan bool) {
	// Define a function to schedule the next reset
	var scheduleReset func()
	scheduleReset = func() {
		now := time.Now().Local()
		nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.Local)
		durationUntilMidnight := nextMidnight.Sub(now)

		// Schedule the ResetDailyMetrics function to execute at midnight
		timer := time.AfterFunc(durationUntilMidnight, func() {
			select {
			case <-stopChan:
				log.Println("Stopping the daily reset scheduler.")
				return
			default:
				// Call ResetDailyMetrics to reset the metrics
				if err := db.ResetDailyMetrics(); err != nil {
					log.Printf("Error resetting daily metrics: %v", err)
				} else {
					log.Println("Successfully reset daily metrics at midnight.")
				}
				// Reschedule for the next midnight
				scheduleReset()
			}
		})

		// Listen on the stopChan to cancel the timer if needed
		go func() {
			<-stopChan
			if !timer.Stop() {
				<-timer.C
			}
			log.Println("Stopping the daily reset scheduler.")
		}()
	}

	// Start the scheduling
	scheduleReset()
}
