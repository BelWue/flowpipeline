package config

// Allows adding Config params that arnt only a simple map
// Needs to be expanded by every Segment using it
type Config struct {
	Config map[string]string `yaml:",inline"`

	//Define custom segment specific structured config params here
	//The parameter MUST contain the segement name to not conflict with other existing config parameters
	ThresholdMetricDefinition []*ThresholdMetricDefinition `yaml:"traffic_specific_toptalkers,omitempty"`
}
