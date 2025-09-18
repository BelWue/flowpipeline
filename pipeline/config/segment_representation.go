package config

import (
	"flag"
	"os"
	"strconv"
)

// A config representation of a segment.
type SegmentRepr struct {
	Name   string `yaml:"segment"`        // to be looked up with a registry
	Config Config `yaml:"config"`         // to be expanded by our instance
	Jobs   int    `yaml:"jobs,omitempty"` // parallel jobs running the pipeline

	//Adds if/then/else
	BranchOptions `yaml:",inline"`

	//Adds matching_pipeline - used to process msg hows IPs are matching a filter
	ThresholdMetricDefinition `yaml:",inline"`
}

// Returns the SegmentRepr's Config with all its variables expanded. It tries
// to match numeric variables such as '$1' to the corresponding command line
// argument not matched by flags, or else uses regular environment variable
// expansion.
func (s *SegmentRepr) ExpandedConfig() map[string]string {
	argvMapper := func(placeholderName string) string {
		argnum, err := strconv.Atoi(placeholderName)
		if err == nil && argnum < len(flag.Args()) {
			return flag.Args()[argnum]
		}
		return ""
	}
	expandedConfig := make(map[string]string)
	for k, v := range s.Config.Config {
		expandedConfig[k] = os.Expand(v, argvMapper) // try to convert $n and such to argv[n]
		if expandedConfig[k] == "" && v != "" {      // if unsuccessful, do regular env expansion
			expandedConfig[k] = os.ExpandEnv(v)
		}
	}
	return expandedConfig
}
