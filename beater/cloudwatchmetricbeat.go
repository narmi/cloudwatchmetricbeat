package beater

import (
	"errors"
	"fmt"
	"time"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/publisher"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"

	"github.com/narmitech/cloudwatchmetricbeat/config"
)

type Cloudwatchmetricbeat struct {
	done   chan struct{}
	config config.Config
	client publisher.Client

	// Client to amazon cloudwatch API
	awsClient cloudwatchiface.CloudWatchAPI

	// AWS client session
	session *session.Session
}

// Creates beater
func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	config := config.DefaultConfig
	if err := cfg.Unpack(&config); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}

	logp.Info(
		"settings: " +
			fmt.Sprintf("config=%+v", config),
	)

	sess := session.Must(session.NewSession(&aws.Config{
		Retryer: client.DefaultRetryer{NumMaxRetries: 10},
		Region:  aws.String(config.AWSRegion),
	}))
	// Create cloudwatch session
	svc := cloudwatch.New(sess)

	beat := &Cloudwatchmetricbeat{
		done:      make(chan struct{}),
		config:    config,
		session:   sess,
		awsClient: svc,
	}
	return beat, nil
}

func (cwb *Cloudwatchmetricbeat) Run(b *beat.Beat) error {
	logp.Info("cloudwatchmetricbeat is running! Hit CTRL-C to stop it.")

	cwb.client = b.Publisher.Connect()
	if cwb.config.Period > 0 {
		go cwb.monitor()
		<-cwb.done
	} else {
		cwb.refreshMetrics()

		for range cwb.config.Prospectors {
			<-cwb.done
		}
	}
	return nil
}

func (b *Cloudwatchmetricbeat) Stop() {
	b.client.Close()
	close(b.done)
}

func (b *Cloudwatchmetricbeat) monitor() {
	ticker := time.NewTicker(b.config.Period)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			b.refreshMetrics()
		}
	}
}

func (b *Cloudwatchmetricbeat) refreshMetrics() {
	for _, configProspector := range b.config.Prospectors {
		p := NewProspector(b, configProspector)
		go func() {
			p.Monitor()
			b.done <- struct{}{} // signal we're done
		}()

		// for _, metricName := range prospector.Metrics {
		// 	metric := NewMetric(prospector.Id, metricName)
		// }
	}
}

// manager

const DefaultPeriodSeconds = 60
const DefaultDelaySeconds = 300
const DefaultRangeSeconds = 600
const DefaultAwsStatistic = "Average"

type Prospector struct {
	config config.Prospector
	beat   *Cloudwatchmetricbeat
}

func NewProspector(beat *Cloudwatchmetricbeat, c config.Prospector) *Prospector {
	return &Prospector{
		config: c,
		beat:   beat,
	}
}

func (p *Prospector) Monitor() {
	for m := range p.config.Metrics {
		metric := &p.config.Metrics[m]
		// validate metric
		if metric.PeriodSeconds == 0 {
			metric.PeriodSeconds = DefaultPeriodSeconds
		}

		if metric.DelaySeconds == 0 {
			metric.DelaySeconds = DefaultDelaySeconds
		}

		if metric.RangeSeconds == 0 {
			metric.RangeSeconds = DefaultRangeSeconds
		}

		if len(metric.AWSStatistics) == 0 {
			metric.AWSStatistics = []string{DefaultAwsStatistic}
		}

		event, err := p.fetchMetric(metric)

		if err == nil {
			opts := []publisher.ClientOption{}
			if p.beat.config.Period <= 0 {
				opts = []publisher.ClientOption{publisher.Sync}
			}
			p.beat.client.PublishEvent(*event, opts...)
		}
	}
}

func (p *Prospector) fetchMetric(metric *config.Metric) (*common.MapStr, error) {
	now := time.Now()
	end := now.Add(time.Duration(-metric.DelaySeconds) * time.Second)

	params := &cloudwatch.GetMetricStatisticsInput{
		EndTime:   aws.Time(end),
		StartTime: aws.Time(end.Add(time.Duration(-metric.RangeSeconds) * time.Second)),

		Period:     aws.Int64(int64(metric.PeriodSeconds)),
		MetricName: aws.String(metric.AWSMetricName),
		Namespace:  aws.String(metric.AWSNamespace),
		Dimensions: []*cloudwatch.Dimension{},
		Statistics: []*string{},
		Unit:       nil,
	}

	for _, stat := range metric.AWSStatistics {
		params.Statistics = append(params.Statistics, aws.String(stat))
	}

	// Loop through the dimensions selects to build the filters
	for dim := range metric.AWSDimensionSelect {
		for val := range metric.AWSDimensionSelect[dim] {
			dimValue := metric.AWSDimensionSelect[dim][val]

			params.Dimensions = append(params.Dimensions, &cloudwatch.Dimension{
				Name:  aws.String(dim),
				Value: aws.String(dimValue),
			})
		}
	}

	// Call CloudWatch to gather the datapoints
	resp, err := p.beat.awsClient.GetMetricStatistics(params)
	logp.Info("making request: " + fmt.Sprintf("params=%+v", params))

	if err != nil {
		logp.Err("aws client error: ", err)
		return nil, err
	}

	// There's nothing in there, don't publish the metric
	if len(resp.Datapoints) == 0 {
		return nil, errors.New("successful call to cloudwatch, but no data")
	}

	// Pick the latest datapoint
	dp := getLatestDatapoint(resp.Datapoints)

	event := common.MapStr{
		"@timestamp":                     common.Time(*dp.Timestamp),
		"type":                           "cloudwatchset",
		"cloudwatchset.name":             p.config.Id,
		"cloudwatchset.resource_id_type": toSnake(*params.Dimensions[0].Name), // TODO handle multiple dimensions
		"cloudwatchset.resource_id":      params.Dimensions[0].Value,
		"cloudwatchset.namespace":        metric.AWSNamespace,
	}

	if dp.Sum != nil {
		event[fmt.Sprintf("%s.sum", toSnake(metric.AWSMetricName))] = float64(*dp.Sum)
	}

	if dp.Average != nil {
		event[fmt.Sprintf("%s.avg", toSnake(metric.AWSMetricName))] = float64(*dp.Average)
	}

	if dp.Maximum != nil {
		event[fmt.Sprintf("%s.max", toSnake(metric.AWSMetricName))] = float64(*dp.Maximum)
	}

	if dp.Minimum != nil {
		event[fmt.Sprintf("%s.min", toSnake(metric.AWSMetricName))] = float64(*dp.Minimum)
	}

	if dp.SampleCount != nil {
		event[fmt.Sprintf("%s.count", toSnake(metric.AWSMetricName))] = float64(*dp.SampleCount)
	}

	return &event, nil
}

func getLatestDatapoint(datapoints []*cloudwatch.Datapoint) *cloudwatch.Datapoint {
	var latest *cloudwatch.Datapoint = nil

	for dp := range datapoints {
		if latest == nil || latest.Timestamp.Before(*datapoints[dp].Timestamp) {
			latest = datapoints[dp]
		}
	}

	return latest
}
