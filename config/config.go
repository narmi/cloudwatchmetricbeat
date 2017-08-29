// Config is put into a different package to prevent cyclic imports in case
// it is needed in several locations

package config

import "time"

type Metric struct {
	AWSStatistics            []string            `config:"aws_statistics"`
	AWSDimensions            []string            `config:"aws_dimensions,omitempty"`
	AWSDimensionSelect       map[string][]string `config:"aws_dimensions_select,omitempty"`
	AWSDimensionsSelectParam map[string][]string `config:"aws_dimensions_select_param,omitempty"`

	AWSNamespace  string `config:"aws_namespace"`
	AWSMetricName string `config:"aws_metric_name"`

	RangeSeconds int `config:"range_seconds,omitempty"`

	//  Set the granularity of the returned datapoints. Must be at least 60
	//  seconds and in multiples of 60.
	PeriodSeconds int `config:"period_seconds,omitempty"`

	DelaySeconds int `config:"delay_seconds,omitempty"`
}

// PeriodSeconds: 60,
// RangeSeconds:  600,
// DelaySeconds: 600,

type Prospector struct {
	Id      string   `config:"id"`
	Metrics []Metric `config:"metrics"`
}

type Config struct {
	// Set how frequently CloudWatch should be queried The default, 900, means
	// check every 15 minutes. Setting this value too low (generally less than
	// 300) results in no metrics being returned from CloudWatch.
	Period      time.Duration `config:"period"`
	AWSRegion   string        `config:"aws_region"`
	Prospectors []Prospector  `config:"prospectors"`
}

var DefaultConfig = Config{
	Period:    60 * time.Second,
	AWSRegion: "us-east-1",
}
