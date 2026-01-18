package grpcclient

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"gometrics/internal/api/metricsdto"
	pb "gometrics/proto/metrics"
)

type Client struct {
	conn    *grpc.ClientConn
	client  pb.MetricsServiceClient
	localIP string
}

func NewClient(addr string, localIP string) (*Client, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:    conn,
		client:  pb.NewMetricsServiceClient(conn),
		localIP: localIP,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// SendMetrics отправляет batch метрик
func (c *Client) SendMetrics(ctx context.Context, metrics []metricsdto.Metrics) error {
	// Добавляем X-Real-IP в metadata
	if c.localIP != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-real-ip", c.localIP)
	}

	pbMetrics := make([]*pb.Metric, 0, len(metrics))
	for _, m := range metrics {
		pm := &pb.Metric{
			Id:    m.ID,
			Mtype: m.MType,
		}
		if m.Value != nil {
			pm.Value = m.Value
		}
		if m.Delta != nil {
			pm.Delta = m.Delta
		}
		pbMetrics = append(pbMetrics, pm)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := c.client.UpdateMetrics(ctx, &pb.UpdateMetricsRequest{
		Metrics: pbMetrics,
	})
	return err
}
