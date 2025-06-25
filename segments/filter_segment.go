// This package is home to all pipeline segment implementations. Generally,
// every segment lives in its own package, implements the Segment interface,
// embeds the BaseSegment to take care of the I/O side of things, and has an
// additional init() function to register itself using RegisterSegment.
package segments

import (
	"github.com/BelWue/flowpipeline/pb"
)

type FilterSegment interface {
	Segment
	SubscribeDrops(drops chan<- *pb.EnrichedFlow) //for processing dropped packages
}

// An extended basis for Segment implementations in the filter group. It
// contains the necessities to process filtered (dropped) flows.
type BaseFilterSegment struct {
	BaseSegment
	Drops chan<- *pb.EnrichedFlow
}

// Set a return channel for dropped flow messages. Segments need to be wary of
// this channel closing when producing messages to this channel. This method is
// only called by the flowpipeline tool from the controlflow/branch segment to
// implement the then/else branches, otherwise this functionality is unused.
func (segment *BaseFilterSegment) SubscribeDrops(drops chan<- *pb.EnrichedFlow) {
	segment.Drops = drops
}
