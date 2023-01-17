//go:build !encore_no_gcp

package gcp

import (
	"io"
	"testing"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/google/go-cmp/cmp"
	"github.com/rs/zerolog"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredres "google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"encore.dev/appruntime/config"
	"encore.dev/metrics"
)

type metricInfo struct {
	name   string
	typ    metrics.MetricType
	svcNum uint16
}

func (m metricInfo) Name() string             { return m.name }
func (m metricInfo) Type() metrics.MetricType { return m.typ }
func (m metricInfo) SvcNum() uint16           { return m.svcNum }

func TestGetMetricData(t *testing.T) {
	newCounterStart := time.Now()
	now := time.Now()
	cfg := &config.GCPCloudMonitoringProvider{
		ProjectID:               "test-project",
		MonitoredResourceType:   "resource-type",
		MonitoredResourceLabels: map[string]string{"key": "value"},
		MetricNames: map[string]string{
			"test_counter": "test_counter",
			"test_gauge":   "test_gauge",
			"test_labels":  "test_labels",
		},
	}
	monitoredRes := &monitoredres.MonitoredResource{
		Type:   "resource-type",
		Labels: map[string]string{"key": "value"},
	}
	pbStart := timestamppb.New(newCounterStart)
	pbEnd := timestamppb.New(now)

	svcs := []string{"foo", "bar"}
	tests := []struct {
		name   string
		metric metrics.CollectedMetric
		data   []*monitoringpb.TimeSeries
	}{
		{
			name: "counter",
			metric: metrics.CollectedMetric{
				Info: metricInfo{"test_counter", metrics.CounterType, 1},
				Val:  int64(10),
			},
			data: []*monitoringpb.TimeSeries{{
				Metric: &metricpb.Metric{
					Type:   "custom.googleapis.com/test_counter",
					Labels: map[string]string{"service": "foo"},
				},
				Resource:   monitoredRes,
				MetricKind: metricpb.MetricDescriptor_CUMULATIVE,
				Points: []*monitoringpb.Point{{
					Interval: &monitoringpb.TimeInterval{StartTime: pbStart, EndTime: pbEnd},
					Value:    int64Val(10),
				}},
			}},
		},
		{
			name: "gauge",
			metric: metrics.CollectedMetric{
				Info: metricInfo{"test_gauge", metrics.GaugeType, 2},
				Val:  float64(0.5),
			},
			data: []*monitoringpb.TimeSeries{{
				Metric: &metricpb.Metric{
					Type:   "custom.googleapis.com/test_gauge",
					Labels: map[string]string{"service": "bar"},
				},
				Resource:   monitoredRes,
				MetricKind: metricpb.MetricDescriptor_GAUGE,
				Points: []*monitoringpb.Point{{
					Interval: &monitoringpb.TimeInterval{EndTime: pbEnd},
					Value:    floatVal(0.5),
				}},
			}},
		},
		{
			name: "labels",
			metric: metrics.CollectedMetric{
				Info:   metricInfo{"test_labels", metrics.GaugeType, 1},
				Labels: []metrics.KeyValue{{"key", "value"}},
				Val:    uint64(2),
			},
			data: []*monitoringpb.TimeSeries{{
				Metric: &metricpb.Metric{
					Type:   "custom.googleapis.com/test_labels",
					Labels: map[string]string{"service": "foo", "key": "value"},
				},
				Resource:   monitoredRes,
				MetricKind: metricpb.MetricDescriptor_GAUGE,
				Points: []*monitoringpb.Point{{
					Interval: &monitoringpb.TimeInterval{EndTime: pbEnd},
					Value:    int64Val(2),
				}},
			}},
		},
		{
			name: "labels_multi_svcs",
			metric: metrics.CollectedMetric{
				Info:   metricInfo{"test_labels", metrics.GaugeType, 0},
				Labels: []metrics.KeyValue{{"key", "value"}},
				Val:    2 * time.Second,
			},
			data: []*monitoringpb.TimeSeries{
				{
					Metric: &metricpb.Metric{
						Type:   "custom.googleapis.com/test_labels",
						Labels: map[string]string{"key": "value"},
					},
					Resource:   monitoredRes,
					MetricKind: metricpb.MetricDescriptor_GAUGE,
					Points: []*monitoringpb.Point{{
						Interval: &monitoringpb.TimeInterval{EndTime: pbEnd},
						Value:    floatVal(2),
					}},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			x := New(svcs, cfg, zerolog.New(io.Discard))
			got := x.getMetricData(newCounterStart, now, []metrics.CollectedMetric{test.metric})
			if diff := cmp.Diff(got, test.data, protocmp.Transform()); diff != "" {
				t.Errorf("getMetricData() mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
