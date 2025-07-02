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

// PushCounter sends a custom counter metric to Google Cloud Monitoring, taking in metric name and the labels for the metric
func PushCounter(ctx context.Context, metricName string, labels map[string]string) {
	initClient(ctx) // Initialize the GCP Monitoring client
	if metricClient == nil {
		return // metrics disabled
	}

	// Get the project ID and function name
	projectID := getProjectID()
	functionName := getFunctionName()
	now := timestamppb.New(time.Now())
	projectName := "projects/" + projectID

	// Always include function_name label for consistency
	if _, ok := labels["function_name"]; !ok {
		labels["function_name"] = functionName
	}

	// Create a point, which is the amount to increment the metric by (1 for now)
	point := &monpb.Point{
		Interval: &monpb.TimeInterval{EndTime: now},
		Value:    &monpb.TypedValue{Value: &monpb.TypedValue_Int64Value{Int64Value: 1}},
	}

	// Create a time series, which is the metric and the point
	ts := &monpb.TimeSeries{
		// Metric is metric name and labels, labels hold the function name and the metric name for the metric
		Metric: &mpb.Metric{
			Type:   "custom.googleapis.com/" + metricName,
			Labels: labels,
		},
		// Resource is the resource that the metric is being collected from, in this case the global resource
		Resource: &gcprpb.MonitoredResource{
			Type: "global", // TODO check global is best
			Labels: map[string]string{
				"project_id": projectID, // No other causese global
			},
		},
		// one per call atm as incrementing by 1
		Points: []*monpb.Point{point},
	}
	// API call to cloud monitoring to log metric
	req := &monpb.CreateTimeSeriesRequest{
		Name:       projectName,
		TimeSeries: []*monpb.TimeSeries{ts},
	}

	if err := metricClient.CreateTimeSeries(ctx, req); err != nil {
		log.Printf("[metrics] could not write time series: %v", err)
	}
}
