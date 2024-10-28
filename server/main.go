package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/qjs/quanti-tea/server/db"
	"github.com/qjs/quanti-tea/server/exporter"
	grpcSrv "github.com/qjs/quanti-tea/server/grpc"
	"github.com/qjs/quanti-tea/server/webapp"

	pb "github.com/qjs/quanti-tea/server/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Command-line flags for configuration
	var (
		dbPath         = flag.String("db", "kettle.db", "Path to SQLite database file")
		grpcPort       = flag.String("grpc-port", ":50051", "gRPC server port")
		prometheusAddr = flag.String("prometheus-addr", ":2112", "Prometheus exporter address")
		webAppPort     = flag.String("webapp-port", ":8005", "Web application port")
	)
	flag.Parse()

	// Initialize Database
	database, err := db.NewDatabase(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Channel to stop the scheduler
	stopChan := make(chan bool)

	// Start the daily reset scheduler
	go database.StartDailyResetScheduler(stopChan)

	// Initialize Prometheus Exporter
	exporter := exporter.NewExporter(database)
	go exporter.Start(*prometheusAddr)

	// Initialize gRPC Server
	lis, err := net.Listen("tcp", *grpcPort)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", *grpcPort, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterMetricsServiceServer(grpcServer, grpcSrv.NewMetricsServer(database))

	// Start gRPC server in a separate goroutine
	go func() {
		log.Printf("gRPC server listening on %s", *grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC server: %v", err)
		}
	}()

	// Wait briefly to ensure gRPC server is up (optional: implement a better synchronization mechanism)
	time.Sleep(1 * time.Second)

	// Establish gRPC client connection
	conn, err := grpc.NewClient("localhost"+*grpcPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()

	grpcClient := pb.NewMetricsServiceClient(conn)

	// Initialize WebApp with gRPC client
	webApp := webapp.NewWebApp(grpcClient)
	go webApp.Run(*webAppPort)

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Println("Shutting down servers...")

	// Create a deadline to wait for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Gracefully shutdown the web server
	webApp.Shutdown(ctx)

	// Gracefully stop the gRPC server
	grpcServer.GracefulStop()

	// Close the gRPC client connection
	if err := conn.Close(); err != nil {
		log.Printf("Failed to close gRPC client connection: %v", err)
	}
	close(stopChan)
	log.Println("Servers shut down successfully.")
}
