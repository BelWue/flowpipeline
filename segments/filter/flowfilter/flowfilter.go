// Runs flows through a filter and forwards only matching flows. Reuses our own
// https://github.com/bwNetFlow/flowfilter project, see the docs there.
package flowfilter

import (
	"log"
	"sync"

	"github.com/bwNetFlow/flowfilter/parser"
	"github.com/bwNetFlow/flowpipeline/pb"
	"github.com/bwNetFlow/flowpipeline/segments"
)

// FIXME: the flowfilter project needs to be updated to new protobuf too
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
		log.Printf("[error] FlowFilter: Syntax error in filter expression: %v", err)
		return nil
	}
	filter := &Filter{}
	if _, err := filter.CheckFlow(newSegment.expression, &pb.EnrichedFlow{}); err != nil {
		log.Printf("[error] FlowFilter: Semantic error in filter expression: %v", err)
		return nil
	}
	return newSegment
}

func (segment *FlowFilter) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	log.Printf("[info] FlowFilter: Using filter expression: %s", segment.Filter)

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
