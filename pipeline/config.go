package pipeline

import (
	"flag"
	"os"
	"strconv"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/segments"
	"github.com/BelWue/flowpipeline/segments/analysis/traffic_specific_toptalkers"
	"github.com/BelWue/flowpipeline/segments/controlflow/branch"
	"gopkg.in/yaml.v2"
)

// A config representation of a segment. It is intended to look like this:
//   - segment: pass
//     config:
//     key: value
//     foo: bar
//
// This struct has the appropriate yaml tags inline.
type SegmentRepr struct {
	Name       string                                                   `yaml:"segment"`               // to be looked up with a registry
	Config     map[string]string                                        `yaml:"config"`                // to be expanded by our instance
	If         []SegmentRepr                                            `yaml:"if,omitempty,flow"`     // only used by group segment
	Then       []SegmentRepr                                            `yaml:"then,omitempty,flow"`   // only used by group segment
	Else       []SegmentRepr                                            `yaml:"else,omitempty,flow"`   // only used by group segment
	Definition []*traffic_specific_toptalkers.ThresholdMetricDefinition `yaml:"definitions,omitempty"` // used to add addition data that is parsed by the segment - e.g. for configs that use complex data
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
	for k, v := range s.Config {
		expandedConfig[k] = os.Expand(v, argvMapper) // try to convert $n and such to argv[n]
		if expandedConfig[k] == "" && v != "" {      // if unsuccessful, do regular env expansion
			expandedConfig[k] = os.ExpandEnv(v)
		}
	}
	return expandedConfig
}

// Builds a list of Segment objects from raw configuration bytes and
// initializes a Pipeline with them.
func NewFromConfig(config []byte) *Pipeline {
	// parse a list of SegmentReprs from yaml
	segmentReprs := new([]SegmentRepr)

	err := yaml.Unmarshal(config, &segmentReprs)
	if err != nil {
		log.Fatal().Err(err).Msg("Error parsing configuration YAML: ")
	}

	segments := SegmentsFromRepr(segmentReprs)

	// we have SegmentReprs parsed, instanciate them as actual Segments
	return New(segments...)
}

// Creates a list of Segments from their config representations. Handles
// recursive definitions found in Segments.
func SegmentsFromRepr(segmentReprs *[]SegmentRepr) []segments.Segment {
	segmentList := make([]segments.Segment, len(*segmentReprs))
	for i, segmentrepr := range *segmentReprs {
		segmentTemplate := segments.LookupSegment(segmentrepr.Name) // a typed nil instance
		// the Segment's New method knows how to handle our config
		segment := segmentTemplate.New(segmentrepr.ExpandedConfig())
		switch segment := segment.(type) { // handle special segments
		case *branch.Branch:
			segment.ImportBranches(
				New(SegmentsFromRepr(&segmentrepr.If)...),
				New(SegmentsFromRepr(&segmentrepr.Then)...),
				New(SegmentsFromRepr(&segmentrepr.Else)...),
			)
		case *traffic_specific_toptalkers.TrafficSpecificToptalkers:
			segment.SetThresholdMetricDefinition(segmentrepr.Definition)
		}
		if segment != nil {
			segmentList[i] = segment
		} else {
			log.Fatal().Msgf("Configured segment '%s' could not be initialized properly, see previous messages.", segmentrepr.Name)
		}
	}
	return segmentList
}
