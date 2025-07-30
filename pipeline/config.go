package pipeline

import (
	"flag"
	"os"
	"strconv"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pipeline/config"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/BelWue/flowpipeline/segments/controlflow/branch"
	"gopkg.in/yaml.v2"
)

// A config representation of a segment.
type SegmentRepr struct {
	Name   string        `yaml:"segment"`             // to be looked up with a registry
	Config config.Config `yaml:"config"`              // to be expanded by our instance
	Jobs   int           `yaml:"jobs,omitempty"`      // parallel jobs running the pipeline
	If     []SegmentRepr `yaml:"if,omitempty,flow"`   // only used by group segment
	Then   []SegmentRepr `yaml:"then,omitempty,flow"` // only used by group segment
	Else   []SegmentRepr `yaml:"else,omitempty,flow"` // only used by group segment
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

// Builds a list of Segment objects from raw configuration bytes and
// initializes a Pipeline with them.
func NewFromConfig(config []byte) *Pipeline {
	// parse a list of SegmentReprs from yaml
	segmentReprs := SegmentReprsFromConfig(config)

	// build segments from it
	segments := SegmentsFromRepr(segmentReprs)

	// we have Segments parsed and ready, instantiate them as actual pipeline
	return New(segments...)
}

// SegmentReprsFromConfig returns a list of segment representation objects from a config.
func SegmentReprsFromConfig(config []byte) []SegmentRepr {
	// parse a list of SegmentReprs from yaml
	segmentReprs := []SegmentRepr{}

	err := yaml.Unmarshal(config, &segmentReprs)
	if err != nil {
		log.Fatal().Err(err).Msg("Error parsing configuration YAML: ")
	}

	return segmentReprs
}

// Creates a list of Segments from their config representations. Handles
// recursive definitions found in Segments.
func SegmentsFromRepr(segmentReprs []SegmentRepr) []segments.Segment {
	segmentList := make([]segments.Segment, len(segmentReprs))
	for i, segmentrepr := range segmentReprs {

		ifPipeline := New(SegmentsFromRepr(segmentrepr.If)...)
		thenPipeline := New(SegmentsFromRepr(segmentrepr.Then)...)
		elsePipeline := New(SegmentsFromRepr(segmentrepr.Else)...)

		segmentTemplate := segments.LookupSegment(segmentrepr.Name) // a typed nil instance

		if segmentrepr.Jobs <= 1 {
			segmentList[i] = segmentFromTemplate(ifPipeline, thenPipeline, elsePipeline, segmentTemplate, segmentrepr)
		} else {
			wrapper := &segments.ParallelizedSegment{}
			for range segmentrepr.Jobs {
				segment := segmentFromTemplate(ifPipeline, thenPipeline, elsePipeline, segmentTemplate, segmentrepr)
				if segment != nil {
					wrapper.AddSegment(segment)
				} else {
					log.Fatal().Msgf("Configured segment '%s' could not be initialized properly, see previous messages.", segmentrepr.Name)
				}
			}
			segmentList[i] = wrapper
		}
	}
	return segmentList
}

func segmentFromTemplate(ifPipeline, thenPipeline, elsePipeline *Pipeline, segmentTemplate segments.Segment, segmentrepr SegmentRepr) segments.Segment {
	// the Segment's New method knows how to handle our config
	segment := segmentTemplate.New(segmentrepr.ExpandedConfig())
	switch segment := segment.(type) { // handle special segments
	case *branch.Branch:
		segment.ImportBranches(
			ifPipeline,
			thenPipeline,
			elsePipeline,
		)
	}
	segment.AddCustomConfig(segmentrepr.Config)
	return segment
}
