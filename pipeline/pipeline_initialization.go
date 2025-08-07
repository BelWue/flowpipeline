package pipeline

import (
	"github.com/BelWue/flowpipeline/pipeline/config"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

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
func SegmentReprsFromConfig(configFile []byte) []config.SegmentRepr {
	// parse a list of SegmentReprs from yaml
	segmentReprs := []config.SegmentRepr{}

	err := yaml.Unmarshal(configFile, &segmentReprs)
	if err != nil {
		log.Fatal().Err(err).Msg("Error parsing configuration YAML: ")
	}

	return segmentReprs
}

// Creates a list of Segments from their config representations. Handles
// recursive definitions found in Segments.
func SegmentsFromRepr(segmentReprs []config.SegmentRepr) []segments.Segment {
	segmentList := make([]segments.Segment, len(segmentReprs))
	for i, segmentrepr := range segmentReprs {
		segmentTemplate := segments.LookupSegment(segmentrepr.Name) // a typed nil instance

		if segmentrepr.Jobs <= 1 {
			segmentList[i] = segmentFromTemplate(segmentTemplate, segmentrepr)
		} else {
			wrapper := &segments.ParallelizedSegment{}
			for range segmentrepr.Jobs {
				segment := segmentFromTemplate(segmentTemplate, segmentrepr)
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

func segmentFromTemplate(segmentTemplate segments.Segment, segmentrepr config.SegmentRepr) segments.Segment {
	// the Segment's New method knows how to handle our config
	segment := segmentTemplate.New(segmentrepr.ExpandedConfig())
	if segment != nil {
		segment.AddCustomConfig(segmentrepr)
	}
	return segment
}
