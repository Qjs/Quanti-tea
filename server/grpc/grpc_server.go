// grpc_server.go
// Handle all of the GRPC connections to the server and protobuf messages
package grpcSrv

import (
	"context"
	"time"

	"github.com/qjs/quanti-tea/server/db"

	pb "github.com/qjs/quanti-tea/server/proto"
)

type MetricsServer struct {
	pb.UnimplementedMetricsServiceServer
	DB *db.Database
}

func NewMetricsServer(database *db.Database) *MetricsServer {
	return &MetricsServer{DB: database}
}

func (s *MetricsServer) AddMetric(ctx context.Context, req *pb.AddMetricRequest) (*pb.AddMetricResponse, error) {
	metric := db.DBMetric{
		MetricName: req.MetricName,
		Type:       req.Type,
		Unit:       req.Unit,
		ResetDaily: req.ResetDaily,
		LastReset:  time.Now(),
	}

	if err := s.DB.AddMetric(metric); err != nil {
		return &pb.AddMetricResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.AddMetricResponse{
		Success: true,
		Message: "Metric added successfully",
	}, nil
}

func (s *MetricsServer) DeleteMetric(ctx context.Context, req *pb.DeleteMetricRequest) (*pb.DeleteMetricResponse, error) {
	err := s.DB.DeleteMetric(req.MetricName)
	if err != nil {
		return &pb.DeleteMetricResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.DeleteMetricResponse{
		Success: true,
		Message: "Metric deleted successfully.",
	}, nil
}

func (s *MetricsServer) IncrementMetric(ctx context.Context, req *pb.IncrementMetricRequest) (*pb.IncrementMetricResponse, error) {
	if err := s.DB.IncrementMetric(req.MetricName, req.Increment); err != nil {
		return &pb.IncrementMetricResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.IncrementMetricResponse{
		Success: true,
		Message: "Metric incremented successfully",
	}, nil
}

func (s *MetricsServer) GetMetrics(ctx context.Context, req *pb.GetMetricsRequest) (*pb.GetMetricsResponse, error) {
	metrics, err := s.DB.GetMetrics()
	if err != nil {
		return nil, err
	}

	var resp pb.GetMetricsResponse
	for _, m := range metrics {
		resp.Metrics = append(resp.Metrics, &pb.Metric{
			MetricName: m.MetricName,
			Type:       m.Type,
			Unit:       m.Unit,
			Value:      m.Value,
			ResetDaily: m.ResetDaily,
			LastReset:  m.LastReset.Format(time.RFC3339),
		})
	}

	return &resp, nil
}

func (s *MetricsServer) UpdateMetric(ctx context.Context, req *pb.UpdateMetricRequest) (*pb.UpdateMetricResponse, error) {
	err := s.DB.UpdateMetric(req.MetricName, req.NewValue)
	if err != nil {
		return &pb.UpdateMetricResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	return &pb.UpdateMetricResponse{
		Success: true,
		Message: "Metric updated successfully.",
	}, nil
}

func (s *MetricsServer) DecrementMetric(ctx context.Context, req *pb.DecrementMetricRequest) (*pb.DecrementMetricResponse, error) {
	err := s.DB.DecrementMetric(req.MetricName, req.Decrement)
	if err != nil {
		return &pb.DecrementMetricResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.DecrementMetricResponse{
		Success: true,
		Message: "Metric decremented successfully.",
	}, nil
}
