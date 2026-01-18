package grpcserver

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"gometrics/internal/api/metricsdto"
	pb "gometrics/proto/metrics"
)

// Service определяет интерфейс для работы с метриками
type Service interface {
	GaugeInsert(ctx context.Context, key string, value float64) error
	CounterInsert(ctx context.Context, key string, value int) error
	GetGauge(ctx context.Context, key string) (float64, error)
	GetCounter(ctx context.Context, key string) (int, error)
	FromStructToStore(ctx context.Context, metric metricsdto.Metrics) error
	FromStructToStoreBatch(ctx context.Context, metrics []metricsdto.Metrics) error
}

type MetricsServer struct {
	pb.UnimplementedMetricsServiceServer
	service       Service
	trustedSubnet *net.IPNet
}

func NewMetricsServer(svc Service, trustedSubnet *net.IPNet) *MetricsServer {
	return &MetricsServer{
		service:       svc,
		trustedSubnet: trustedSubnet,
	}
}

// UpdateMetric обновляет одну метрику
func (s *MetricsServer) UpdateMetric(ctx context.Context, m *pb.Metric) (*pb.Metric, error) {
	if err := s.checkTrustedSubnet(ctx); err != nil {
		return nil, err
	}

	switch m.Mtype {
	case metricsdto.MetricTypeGauge:
		if m.Value == nil {
			return nil, status.Error(codes.InvalidArgument, "value is required for gauge")
		}
		if err := s.service.GaugeInsert(ctx, m.Id, *m.Value); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		val, _ := s.service.GetGauge(ctx, m.Id)
		return &pb.Metric{Id: m.Id, Mtype: m.Mtype, Value: &val}, nil

	case metricsdto.MetricTypeCounter:
		if m.Delta == nil {
			return nil, status.Error(codes.InvalidArgument, "delta is required for counter")
		}
		if err := s.service.CounterInsert(ctx, m.Id, int(*m.Delta)); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		val, _ := s.service.GetCounter(ctx, m.Id)
		delta := int64(val)
		return &pb.Metric{Id: m.Id, Mtype: m.Mtype, Delta: &delta}, nil

	default:
		return nil, status.Error(codes.InvalidArgument, "unknown metric type")
	}
}

// UpdateMetrics batch обновление метрик
func (s *MetricsServer) UpdateMetrics(ctx context.Context, req *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
	if err := s.checkTrustedSubnet(ctx); err != nil {
		return nil, err
	}

	metrics := make([]metricsdto.Metrics, 0, len(req.Metrics))
	for _, m := range req.Metrics {
		metric := metricsdto.Metrics{
			ID:    m.Id,
			MType: m.Mtype,
			Value: m.Value,
		}
		if m.Delta != nil {
			metric.Delta = m.Delta
		}
		metrics = append(metrics, metric)
	}

	if err := s.service.FromStructToStoreBatch(ctx, metrics); err != nil {
		return &pb.UpdateMetricsResponse{Success: false, Error: err.Error()}, nil
	}

	return &pb.UpdateMetricsResponse{Success: true}, nil
}

// GetMetric получение метрики
func (s *MetricsServer) GetMetric(ctx context.Context, req *pb.GetMetricRequest) (*pb.Metric, error) {
	switch req.Mtype {
	case metricsdto.MetricTypeGauge:
		val, err := s.service.GetGauge(ctx, req.Id)
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return &pb.Metric{Id: req.Id, Mtype: req.Mtype, Value: &val}, nil

	case metricsdto.MetricTypeCounter:
		val, err := s.service.GetCounter(ctx, req.Id)
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		delta := int64(val)
		return &pb.Metric{Id: req.Id, Mtype: req.Mtype, Delta: &delta}, nil

	default:
		return nil, status.Error(codes.InvalidArgument, "unknown metric type")
	}
}

// checkTrustedSubnet проверка IP из metadata
func (s *MetricsServer) checkTrustedSubnet(ctx context.Context) error {
	if s.trustedSubnet == nil {
		return nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil // нет metadata — пропускаем
	}

	realIP := md.Get("x-real-ip")
	if len(realIP) == 0 {
		return nil // нет заголовка — пропускаем
	}

	ip := net.ParseIP(realIP[0])
	if ip == nil {
		return status.Error(codes.PermissionDenied, "invalid IP in X-Real-IP")
	}

	if !s.trustedSubnet.Contains(ip) {
		return status.Error(codes.PermissionDenied, fmt.Sprintf("IP %s not in trusted subnet", ip))
	}

	return nil
}

// Run запускает gRPC сервер
func Run(addr string, svc Service, trustedSubnet *net.IPNet) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	srv := grpc.NewServer()
	pb.RegisterMetricsServiceServer(srv, NewMetricsServer(svc, trustedSubnet))

	return srv.Serve(lis)
}
