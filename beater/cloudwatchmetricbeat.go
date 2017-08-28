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

	"github.com/phillbaker/cloudwatchmetricbeat/config"
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
	// ticker := time.NewTicker(cwb.config.Period)
	// counter := 1
	// for {
	// 	select {
	// 	case <-cwb.done:
	// 		return nil
	// 	case <-ticker.C:
	// 	}

	// 	event := common.MapStr{
	// 		"@timestamp": common.Time(time.Now()),
	// 		"type":       b.Name,
	// 		"counter":    counter,
	// 	}
	// 	cwb.client.PublishEvent(event)
	// 	logp.Info("Event sent")
	// 	counter++
	// }
	go cwb.monitor()
	<-cwb.done
	return nil
}

func (b *Cloudwatchmetricbeat) Stop() {
	b.client.Close()
	close(b.done)
}

// manager

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
		go p.Monitor()
		// for _, metricName := range prospector.Metrics {
		// 	metric := NewMetric(prospector.Id, metricName)

		// }
	}
}

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
			metric.PeriodSeconds = 60
		}
		event, err := p.fetchMetric(metric)

		if err == nil {
			p.beat.client.PublishEvent(*event)
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

	// labels := make([]string, 0, len(metric.LabelNames))

	// Loop through the dimensions selects to build the filters and the labels array
	for dim := range metric.AWSDimensionSelect {
		for val := range metric.AWSDimensionSelect[dim] {
			dimValue := metric.AWSDimensionSelect[dim][val]

			params.Dimensions = append(params.Dimensions, &cloudwatch.Dimension{
				Name:  aws.String(dim),
				Value: aws.String(dimValue),
			})

			// labels = append(labels, dimValue)
		}
	}

	// labels = append(labels, collector.Template.Task.Name)

	// Call CloudWatch to gather the datapoints
	resp, err := p.beat.awsClient.GetMetricStatistics(params)
	logp.Info("making request: " + fmt.Sprintf("params=%+v", params))
	// totalRequests.Inc()

	if err != nil {
		// collector.ErroneousRequests.Inc()
		logp.Err("aws client error: ", err)
		return nil, err
	}

	// There's nothing in there, don't publish the metric
	if len(resp.Datapoints) == 0 {
		return nil, errors.New("successful call to cloudwatch, but no data")
	}

	// Pick the latest datapoint
	// dp := getLatestDatapoint(resp.Datapoints) // TODO
	// logp.Info("Data=%+v", resp.Datapoints)
	dp := resp.Datapoints[0]
	var value float64
	if dp.Sum != nil {
		value = float64(*dp.Sum)
	}

	if dp.Average != nil {
		value = float64(*dp.Average)
	}

	if dp.Maximum != nil {
		value = float64(*dp.Maximum)
	}

	if dp.Minimum != nil {
		value = float64(*dp.Minimum)
	}

	if dp.SampleCount != nil {
		value = float64(*dp.SampleCount)
	}

	return &common.MapStr{
		"@timestamp":              common.Time(time.Now()),
		"cloudwatch.metric_group": p.config.Id,
		"cloudwatch.resource":     metric.AWSDimensionSelect, // I believe there has to be exactly one?
		"cloudwatch.metric_type":  metric.AWSNamespace,
		"cloudwatch.metric_name":  metric.AWSMetricName,
		"cloudwatch.value":        value,
	}, nil
}
