[![Travis Build Status](https://travis-ci.org/narmitech/cloudwatchmetricbeat.svg?branch=master)](https://travis-ci.org/narmitech/cloudwatchmetricbeat)

# Cloudwatchmetricbeat

Welcome to Cloudwatchmetricbeat. Based on previous work on [Cloudwatch Logs](https://github.com/e-travel/cloudwatchlogsbeat) and [Metrics](https://github.com/Technofy/cloudwatch_exporter)

## Installation

[Download a binary](https://github.com/narmitech/cloudwatchmetricbeat/releases), and put it in a good spot on your system. Or use the [Dockerfile](Dockerfile) to build a Docker image.

To run this beat with a given [configuration](#configuration), run:

```
./cloudwatchmetricbeat -c cloudwatchmetricbeat.yml -e -d "*"
```


## Credentials and permissions

The CloudWatch Exporter uses the
[AWS Go SDK](http://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/welcome.html),
which offers [a variety of ways to provide credentials](http://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/sessions.html#creating-sessions).
This includes the `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` environment
variables.

The `cloudwatch:ListMetrics` and `cloudwatch:GetMetricStatistics` IAM permissions are required.

## Configuration

The configuration is in YAML, an example with common options:
```
---
aws_region: us-east-1
metrics:
 - aws_namespace: AWS/ELB
   aws_metric_name: RequestCount
   aws_dimensions: [AvailabilityZone, LoadBalancerName]
   aws_dimension_select:
     LoadBalancerName: [myLB]
   aws_statistics: [Sum]
```

Name     | Description
---------|------------
aws_region   | Required. The AWS region to connect to.
metrics  | Required. A list of CloudWatch metrics to retrieve and export
aws_namespace  | Required. Namespace of the CloudWatch metric.
aws_metric_name  | Required. Metric name of the CloudWatch metric.
aws_dimensions | Optional. Which dimension to fan out over.
aws_dimension_select | Optional. Which dimension values to filter. Specify a map from the dimension name to a list of values to select from that dimension.
aws_statistics | Optional. A list of statistics to retrieve, values can include Sum, SampleCount, Minimum, Maximum, Average. Defaults to Average.
delay_seconds | Optional. The newest data to request. Used to avoid collecting data that has not fully converged. Defaults to 300s.
range_seconds | Optional. How far back to request data for. Useful for cases such as Billing metrics that are only set every few hours. Defaults to 600s.
period_seconds | Optional. [Period](http://docs.aws.amazon.com/AmazonCloudWatch/latest/DeveloperGuide/cloudwatch_concepts.html#CloudWatchPeriods) to request the metric for. Only the most recent data point is used. Defaults to 60s. Can be set globally and per metric.


CloudWatch has been observed to sometimes take minutes for reported values to
converge. The default `delay_seconds` will result in data that is at least 5
minutes old being requested to mitigate this.

* Data points with a period of less than 60 seconds are available for
3 hours. These data points are high-resolution metrics and are available
only for custom metrics that have been defined with a StorageResolution
of 1.
* Data points with a period of 60 seconds (1-minute) are available for
15 days.
* Data points with a period of 300 seconds (5-minute) are available for
63 days.
* Data points with a period of 3600 seconds (1 hour) are available for
455 days (15 months).

### Cost

Amazon charges for every API request, see the [current charges](http://aws.amazon.com/cloudwatch/pricing/).

Every metric retrieved requires one API request, which can include multiple
statistics. In addition, when `aws_dimensions` is provided, the exporter needs
to do API requests to determine what metrics to request. This should be
negligible compared to the requests for the metrics themselves.

If you have 100 API requests every minute, with the price of USD$10 per million
requests (as of Jan 2015), that is around $45 per month.

## Getting Started with Cloudwatchmetricbeat

### Requirements

* [Golang](https://golang.org/dl/) >= 1.7
* [Glide](https://github.com/Masterminds/glide)

### Init Project

Clone this repository at the following location: `${GOPATH}/github.com/narmitech/cloudwatchmetricbeat`.

To get running with Cloudwatchmetricbeat and also install the
dependencies, run the following command:

```
make setup
```

It will create a clean git history for each major step. Note that you can always rewrite the history if you wish before pushing your changes.

To push Cloudwatchmetricbeat in the git repository, run the following commands:

```
git remote set-url origin https://github.com/narmitech/cloudwatchmetricbeat
git push origin master
```

For further development, check out the [beat developer guide](https://www.elastic.co/guide/en/beats/libbeat/current/new-beat.html).

### Build

To build the binary for Cloudwatchmetricbeat run the command below. This will generate a binary in the same directory with the name cloudwatchmetricbeat.

```
make
```


### Run

To run Cloudwatchmetricbeat with debugging output enabled, run:

```
./cloudwatchmetricbeat -c cloudwatchmetricbeat.yml -e -d "*"
```


### Test

To test Cloudwatchmetricbeat, run the following command:

```
make testsuite
```

alternatively:
```
make unit-tests
make system-tests
make integration-tests
make coverage-report
```

The test coverage is reported in the folder `./build/coverage/`

### Update

Each beat has a template for the mapping in elasticsearch and a documentation for the fields which is automatically generated based on `etc/fields.yml`.
To generate etc/cloudwatchmetricbeat.template.json and etc/cloudwatchmetricbeat.asciidoc

```
make update
```


### Cleanup

To clean  Cloudwatchmetricbeat source code, run the following commands:

```
make fmt
make simplify
```

To clean up the build directory and generated artifacts, run:

```
make clean
```


### Clone

To clone Cloudwatchmetricbeat from the git repository, run the following commands:

```
mkdir -p ${GOPATH}/github.com/narmitech/cloudwatchmetricbeat
cd ${GOPATH}/github.com/narmitech/cloudwatchmetricbeat
git clone https://github.com/narmitech/cloudwatchmetricbeat
```


For further development, check out the [beat developer guide](https://www.elastic.co/guide/en/beats/libbeat/current/new-beat.html).


## Packaging

The beat frameworks provides tools to crosscompile and package your beat for different platforms. This requires [docker](https://www.docker.com/) and vendoring as described above. To build packages of your beat, run the following command:

```
make package
```

This will fetch and create all images required for the build process. The whole process to finish can take several minutes.
