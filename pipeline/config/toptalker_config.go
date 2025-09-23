package config

import a "github.com/BelWue/flowpipeline/pipeline/config/evaluation_mode"

type ThresholdMetricDefinition struct {
	MatchingPipeline []SegmentRepr `yaml:"matching_pipeline,omitempty"`
}

type ThresholdMetricConfig struct {
	PrometheusMetricsParamsDefinition `yaml:",inline"`
	FilterDefinition                  string                   `yaml:"filter,omitempty"`
	SubDefinitions                    []*ThresholdMetricConfig `yaml:"subfilter,omitempty"`
}

type PrometheusMetricsParamsDefinition struct {
	TrafficType      string           `yaml:"traffictype,omitempty"`      // optional, default is "", name for the traffic type (included as label)
	Buckets          int              `yaml:"buckets,omitempty"`          // optional, default is 60, sets the number of seconds used as a sliding window size
	ThresholdBuckets int              `yaml:"thresholdbuckets,omitempty"` // optional, use the last N buckets for calculation of averages, default: $Buckets
	ReportBuckets    int              `yaml:"reportbuckets,omitempty"`    // optional, use the last N buckets to calculate averages that are reported as result, default: $Buckets
	BucketDuration   int              `yaml:"bucketduration,omitempty"`   // optional, duration of a bucket, default is 1 second
	ThresholdBps     uint64           `yaml:"thresholdbps,omitempty"`     // optional, default is 0, only log talkers with an average bits per second rate higher than this value
	ThresholdPps     uint64           `yaml:"thresholdpps,omitempty"`     // optional, default is 0, only log talkers with an average packets per second rate higher than this value
	RelevantAddress  a.EvaluationMode `yaml:"relevantaddress,omitempty"`  // optional deprecated replaced by evaluationmode, default is "destination", options are "destination", "source", "both", "connection"
	EvaluationMode   a.EvaluationMode `yaml:"evaluationmode,omitempty"`   // optional, default is "destination", options are "destination", "source", "source and destination", "connection"
}
