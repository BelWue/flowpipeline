// Runs flows through a filter and forwards only matching flows. Reuses our own
// https://github.com/BelWue/flowfilter project, see the docs there.
package flowfilter

import (
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowfilter/parser"
	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
)

type FlowFilter struct {
	segments.BaseFilterSegment
	Filter string // optional, default is empty

	expression *parser.Expression
}

func (segment FlowFilter) New(config map[string]string) segments.Segment {
	var err error

	newSegment := &FlowFilter{
		Filter: config["filter"],
	}

	newSegment.expression, err = parser.Parse(config["filter"])
	if err != nil {
		log.Error().Err(err).Msg("FlowFilter: Syntax error in filter expression: ")
		return nil
	}
	filter := &Filter{}
	if _, err := filter.CheckFlow(newSegment.expression, &pb.EnrichedFlow{}); err != nil {
		log.Error().Err(err).Msg("FlowFilter: Semantic error in filter expression: ")
		return nil
	}
	return newSegment
}

func (segment *FlowFilter) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	log.Info().Msgf("FlowFilter: Using filter expression: %s", segment.Filter)

	filter := &Filter{}
	for msg := range segment.In {
		if match, _ := filter.CheckFlow(segment.expression, msg); match {
			segment.Out <- msg
		} else if segment.Drops != nil {
			segment.Drops <- msg
		}
	}
}

func init() {
	segment := &FlowFilter{}
	segments.RegisterSegment("flowfilter", segment)
}
