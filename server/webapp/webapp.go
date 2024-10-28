package webapp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	pb "github.com/qjs/quanti-tea/server/proto"
)

// WebApp encapsulates the Gin engine and gRPC client reference
type WebApp struct {
	Router     *gin.Engine
	GRPCClient pb.MetricsServiceClient
	Server     *http.Server
}

// NewWebApp initializes the web application with routes and templates
func NewWebApp(grpcClient pb.MetricsServiceClient) *WebApp {
	router := gin.Default()
	router.LoadHTMLGlob("server/webapp/templates/*")

	app := &WebApp{
		Router:     router,
		GRPCClient: grpcClient,
	}

	app.setupRoutes()
	return app
}

// setupRoutes defines all the HTTP routes for the web application
func (app *WebApp) setupRoutes() {
	app.Router.GET("/", app.getMetrics)
	app.Router.POST("/add", app.addMetric)
	app.Router.POST("/delete", app.deleteMetric)
	app.Router.POST("/update", app.updateMetric)
	app.Router.POST("/increment", app.incrementMetric)
	app.Router.POST("/decrement", app.decrementMetric)
}

// getMetrics handles GET requests to display all metrics
func (app *WebApp) getMetrics(c *gin.Context) {
	metrics, err := app.fetchMetrics(c)
	if err != nil {
		// Error already handled in fetchMetrics
		return
	}

	c.HTML(http.StatusOK, "index.html", gin.H{
		"Metrics": metrics,
	})
}

// addMetric handles POST requests to add a new metric
func (app *WebApp) addMetric(c *gin.Context) {
	metricName := c.PostForm("metric_name")
	metricType := c.PostForm("metric_type")
	metricUnit := c.PostForm("metric_unit")
	resetDaily := c.PostForm("reset_daily") == "on"

	// Validate input
	if metricName == "" || metricType == "" {
		metrics, _ := app.fetchMetrics(c) // Fetch metrics even on error
		c.HTML(http.StatusBadRequest, "index.html", gin.H{
			"Metrics": metrics,
			"Error":   "Metric name and type are required.",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.AddMetricRequest{
		MetricName: metricName,
		Type:       metricType,
		Unit:       metricUnit,
		ResetDaily: resetDaily,
	}

	resp, err := app.GRPCClient.AddMetric(ctx, req)
	if err != nil {
		log.Printf("AddMetric RPC failed: %v", err)
		metrics, _ := app.fetchMetrics(c)
		c.HTML(http.StatusInternalServerError, "index.html", gin.H{
			"Metrics": metrics,
			"Error":   fmt.Sprintf("Failed to add metric: %v", err),
		})
		return
	}

	if !resp.Success {
		metrics, _ := app.fetchMetrics(c)
		c.HTML(http.StatusBadRequest, "index.html", gin.H{
			"Metrics": metrics,
			"Error":   resp.Message,
		})
		return
	}

	// Fetch updated metrics
	metrics, err := app.fetchMetrics(c)
	if err != nil {
		// Error already handled in fetchMetrics
		return
	}

	c.HTML(http.StatusOK, "index.html", gin.H{
		"Metrics": metrics,
		"Message": "Metric added successfully.",
	})
}

// addMetric handles POST requests to add a new metric
func (app *WebApp) deleteMetric(c *gin.Context) {
	metricName := c.PostForm("metric_name")

	// Validate input
	if metricName == "" {
		metrics, _ := app.fetchMetrics(c) // Fetch metrics even on error
		c.HTML(http.StatusBadRequest, "index.html", gin.H{
			"Metrics": metrics,
			"Error":   "Metric name and type are required.",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.DeleteMetricRequest{
		MetricName: metricName,
	}

	resp, err := app.GRPCClient.DeleteMetric(ctx, req)
	if err != nil {
		log.Printf("DeleteMetric RPC failed: %v", err)
		metrics, _ := app.fetchMetrics(c)
		c.HTML(http.StatusInternalServerError, "index.html", gin.H{
			"Metrics": metrics,
			"Error":   fmt.Sprintf("Failed to delete metric: %v", err),
		})
		return
	}

	if !resp.Success {
		metrics, _ := app.fetchMetrics(c)
		c.HTML(http.StatusBadRequest, "index.html", gin.H{
			"Metrics": metrics,
			"Error":   resp.Message,
		})
		return
	}

	// Fetch updated metrics
	metrics, err := app.fetchMetrics(c)
	if err != nil {
		// Error already handled in fetchMetrics
		return
	}

	c.HTML(http.StatusOK, "index.html", gin.H{
		"Metrics": metrics,
		"Message": "Metric deleted successfully.",
	})
}

// updateMetric handles POST requests to update a metric's value
func (app *WebApp) updateMetric(c *gin.Context) {
	metricName := c.PostForm("metric_name")
	newValueStr := c.PostForm("new_value")

	var newValue float64
	_, err := fmt.Sscanf(newValueStr, "%f", &newValue)
	if err != nil {
		metrics, _ := app.fetchMetrics(c)
		c.HTML(http.StatusBadRequest, "index.html", gin.H{
			"Metrics": metrics,
			"Error":   "Invalid value for update.",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.UpdateMetricRequest{
		MetricName: metricName,
		NewValue:   newValue,
	}

	resp, err := app.GRPCClient.UpdateMetric(ctx, req)
	if err != nil {
		log.Printf("UpdateMetric RPC failed: %v", err)
		metrics, _ := app.fetchMetrics(c)
		c.HTML(http.StatusInternalServerError, "index.html", gin.H{
			"Metrics": metrics,
			"Error":   fmt.Sprintf("Failed to update metric: %v", err),
		})
		return
	}

	if !resp.Success {
		metrics, _ := app.fetchMetrics(c)
		c.HTML(http.StatusBadRequest, "index.html", gin.H{
			"Metrics": metrics,
			"Error":   resp.Message,
		})
		return
	}

	// Fetch updated metrics
	metrics, err := app.fetchMetrics(c)
	if err != nil {
		// Error already handled in fetchMetrics
		return
	}

	c.HTML(http.StatusOK, "index.html", gin.H{
		"Metrics": metrics,
		"Message": "Metric updated successfully.",
	})
}

// incrementMetric handles POST requests to increment a metric's value
func (app *WebApp) incrementMetric(c *gin.Context) {
	metricName := c.PostForm("metric_name")

	if metricName == "" {
		metrics, _ := app.fetchMetrics(c)
		c.HTML(http.StatusBadRequest, "index.html", gin.H{
			"Metrics": metrics,
			"Error":   "Metric name is required for increment.",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.IncrementMetricRequest{
		MetricName: metricName,
		Increment:  1, // Increment by 1, not all metrics are a +1 incrementer, TODO: make it an adjustable implementation
	}

	resp, err := app.GRPCClient.IncrementMetric(ctx, req)
	if err != nil {
		log.Printf("IncrementMetric RPC failed: %v", err)
		metrics, _ := app.fetchMetrics(c)
		c.HTML(http.StatusInternalServerError, "index.html", gin.H{
			"Metrics": metrics,
			"Error":   fmt.Sprintf("Failed to increment metric: %v", err),
		})
		return
	}

	if !resp.Success {
		metrics, _ := app.fetchMetrics(c)
		c.HTML(http.StatusBadRequest, "index.html", gin.H{
			"Metrics": metrics,
			"Error":   resp.Message,
		})
		return
	}

	// Fetch updated metrics
	metrics, err := app.fetchMetrics(c)
	if err != nil {
		// Error already handled in fetchMetrics
		return
	}

	c.HTML(http.StatusOK, "index.html", gin.H{
		"Metrics": metrics,
		"Message": "Metric incremented successfully.",
	})
}

// decrementMetric handles POST requests to decrement a metric's value
func (app *WebApp) decrementMetric(c *gin.Context) {
	metricName := c.PostForm("metric_name")

	if metricName == "" {
		metrics, _ := app.fetchMetrics(c)
		c.HTML(http.StatusBadRequest, "index.html", gin.H{
			"Metrics": metrics,
			"Error":   "Metric name is required for decrement.",
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.DecrementMetricRequest{
		MetricName: metricName,
		Decrement:  1, // Decrement by 1
	}

	resp, err := app.GRPCClient.DecrementMetric(ctx, req)
	if err != nil {
		log.Printf("DecrementMetric RPC failed: %v", err)
		metrics, _ := app.fetchMetrics(c)
		c.HTML(http.StatusInternalServerError, "index.html", gin.H{
			"Metrics": metrics,
			"Error":   fmt.Sprintf("Failed to decrement metric: %v", err),
		})
		return
	}

	if !resp.Success {
		metrics, _ := app.fetchMetrics(c)
		c.HTML(http.StatusBadRequest, "index.html", gin.H{
			"Metrics": metrics,
			"Error":   resp.Message,
		})
		return
	}

	// Fetch updated metrics
	metrics, err := app.fetchMetrics(c)
	if err != nil {
		// Error already handled in fetchMetrics
		return
	}

	c.HTML(http.StatusOK, "index.html", gin.H{
		"Metrics": metrics,
		"Message": "Metric decremented successfully.",
	})
}

// fetchMetrics is a helper function to retrieve metrics via gRPC and handle errors
func (app *WebApp) fetchMetrics(c *gin.Context) ([]*pb.Metric, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := app.GRPCClient.GetMetrics(ctx, &pb.GetMetricsRequest{})
	if err != nil {
		log.Printf("GetMetrics RPC failed: %v", err)
		c.HTML(http.StatusInternalServerError, "index.html", gin.H{
			"Error": fmt.Sprintf("Failed to fetch metrics: %v", err),
		})
		return nil, err
	}

	return resp.Metrics, nil
}

// Run starts the Gin web server
func (app *WebApp) Run(addr string) {
	app.Server = &http.Server{
		Addr:    addr,
		Handler: app.Router,
	}

	go func() {
		log.Printf("Starting web server on %s", addr)
		if err := app.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to run Gin server: %v", err)
		}
	}()
}

// Shutdown gracefully shuts down the web server without interrupting active connections
func (app *WebApp) Shutdown(ctx context.Context) {
	if app.Server != nil {
		log.Println("Shutting down web server...")
		if err := app.Server.Shutdown(ctx); err != nil {
			log.Fatalf("Web server Shutdown Failed:%+v", err)
		}
		log.Println("Web server exited properly")
	}
}
