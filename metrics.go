package metrics

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	mpb "google.golang.org/genproto/googleapis/api/metric"
	gcprpb "google.golang.org/genproto/googleapis/api/monitoredres"
	monpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Global variables for the GCP Monitoring client
var (
	clientInit   sync.Once
	metricClient *monitoring.MetricClient
	clientErr    error
)

// getProjectID returns the GCP project ID from env or default to p48-development for local
func getProjectID() string {
	if v := os.Getenv("GOOGLE_CLOUD_PROJECT"); v != "" {
		return v
	}
	return "p48-development"
}

// getFunctionName returns the function name from env or default to Buy cause of video-game-shop
func getFunctionName() string {
	if v := os.Getenv("FUNCTION_NAME"); v != "" {
		return v
	}
	return "Buy" // TODO prob change this
}

// initClient initializes the GCP Monitoring client once
func initClient(ctx context.Context) {
	clientInit.Do(func() {
		metricClient, clientErr = monitoring.NewMetricClient(ctx) // Connection to cloud monitoring
		if clientErr != nil {
			log.Printf("[metrics] disabled – failed to create Monitoring client: %v", clientErr)
		}
	})
}

// PushMetric sends a custom metric with any value type to Google Cloud Monitoring
func PushMetric(ctx context.Context, metricName string, value interface{}, labels map[string]string) {
	initClient(ctx) // Initialize the GCP Monitoring client
	if metricClient == nil {
		return // metrics disabled
	}

	projectID := getProjectID()
	functionName := getFunctionName()
	now := timestamppb.New(time.Now())
	projectName := "projects/" + projectID

	// Always include function_name label for consistency
	if _, ok := labels["function_name"]; !ok {
		labels["function_name"] = functionName
	}

	// Create a typed value for the metric - allows for different types of values
	var typedValue *monpb.TypedValue
	switch v := value.(type) {
	case int:
		typedValue = &monpb.TypedValue{Value: &monpb.TypedValue_Int64Value{Int64Value: int64(v)}}
	case int32:
		typedValue = &monpb.TypedValue{Value: &monpb.TypedValue_Int64Value{Int64Value: int64(v)}}
	case int64:
		typedValue = &monpb.TypedValue{Value: &monpb.TypedValue_Int64Value{Int64Value: v}}
	case float32:
		typedValue = &monpb.TypedValue{Value: &monpb.TypedValue_DoubleValue{DoubleValue: float64(v)}}
	case float64:
		typedValue = &monpb.TypedValue{Value: &monpb.TypedValue_DoubleValue{DoubleValue: v}}
	case string:
		typedValue = &monpb.TypedValue{Value: &monpb.TypedValue_StringValue{StringValue: v}}
	case bool:
		var intVal int64
		if v {
			intVal = 1
		}
		typedValue = &monpb.TypedValue{Value: &monpb.TypedValue_Int64Value{Int64Value: intVal}}
	default:
		log.Printf("[metrics] unsupported value type: %T", v)
		return
	}

	point := &monpb.Point{
		Interval: &monpb.TimeInterval{EndTime: now},
		Value:    typedValue,
	}

	ts := &monpb.TimeSeries{
		Metric: &mpb.Metric{
			Type:   "custom.googleapis.com/" + metricName,
			Labels: labels,
		},
		Resource: &gcprpb.MonitoredResource{
			Type: "global",
			Labels: map[string]string{
				"project_id": projectID,
			},
		},
		Points: []*monpb.Point{point},
	}

	req := &monpb.CreateTimeSeriesRequest{
		Name:       projectName,
		TimeSeries: []*monpb.TimeSeries{ts},
	}

	if err := metricClient.CreateTimeSeries(ctx, req); err != nil {
		log.Printf("[metrics] could not write time series: %v", err)
	}
}
