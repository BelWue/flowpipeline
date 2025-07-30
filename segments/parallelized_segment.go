// This package is home to all pipeline segment implementations. Generally,
// every segment lives in its own package, implements the Segment interface,
// embeds the BaseSegment to take care of the I/O side of things, and has an
// additional init() function to register itself using RegisterSegment.
package segments

import (
	"sync"

	"github.com/BelWue/flowpipeline/pb"
)

// Wrapper allowing multiple parallel instances of a segment by wiring in/out/drops-channels to all contained segments
type ParallelizedSegment struct {
	BaseFilterSegment
	segments []Segment
}

func (segment *ParallelizedSegment) New(config map[string]string) Segment {
	// This method should never be called, since ParallelizedSegment is just a wrapper for other segmets
	panic("ParallelizedSegment should not be instantiated using New()")
}

func (segment *ParallelizedSegment) Close() {
	for _, segment := range segment.segments {
		segment.Close()
	}
}

func (segment *ParallelizedSegment) SubscribeDrops(drop chan *pb.EnrichedFlow) {
	for _, nestedSegment := range segment.segments {
		filterSegment, ok := nestedSegment.(FilterSegment)
		if ok {
			filterSegment.SubscribeDrops(drop)
		}
	}
}

func (segment *ParallelizedSegment) Run(wg *sync.WaitGroup) {
	defer wg.Done()
	segmentWg := sync.WaitGroup{}
	for _, segment := range segment.segments {
		segmentWg.Add(1)
		go segment.Run(&segmentWg)
	}
	segmentWg.Wait()
}

func (segment *ParallelizedSegment) Rewire(in chan *pb.EnrichedFlow, out chan *pb.EnrichedFlow) {
	for _, segment := range segment.segments {
		segment.Rewire(in, out)
	}
}

func (segment *ParallelizedSegment) AddSegment(nestedSegment Segment) {
	segment.segments = append(segment.segments, nestedSegment)
}

// ShutdownParentPipeline implements Segment.
func (segment *ParallelizedSegment) ShutdownParentPipeline() {
	for _, segment := range segment.segments {
		segment.ShutdownParentPipeline()
	}
}
