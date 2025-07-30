// The pipeline package manages segments in Pipeline objects.
package pipeline

import (
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/BelWue/flowpipeline/segments/pass"
)

// Basically a list of segments. It further exposes the In and Out channels of
// the Pipeline as a whole, i.e. the ingress channel of the first and the
// egress channel of the last segment in its SegmentList.
type Pipeline struct {
	In          chan *pb.EnrichedFlow
	Out         <-chan *pb.EnrichedFlow
	Drop        chan *pb.EnrichedFlow
	wg          *sync.WaitGroup
	SegmentList []segments.Segment
}

func (pipeline *Pipeline) GetInput() chan *pb.EnrichedFlow {
	return pipeline.In
}

func (pipeline *Pipeline) GetOutput() <-chan *pb.EnrichedFlow {
	return pipeline.Out
}

func (pipeline *Pipeline) GetDrop() <-chan *pb.EnrichedFlow {
	if pipeline.Drop != nil {
		return pipeline.Drop
	}
	pipeline.Drop = make(chan *pb.EnrichedFlow)
	// Subscribe to drops from special segments, namely all based on
	// BaseFilterSegment grouped in the filter directory.
	for _, segment := range pipeline.SegmentList {
		value, implementsFilter := segment.(segments.FilterSegment)
		if implementsFilter {
			value.SubscribeDrops(pipeline.Drop)
		}
	}
	// If there are no filter/* segments, this channel will never have
	// messages available.
	return pipeline.Drop
}

// Starts up a goroutine specific to this Pipeline which reads any message from
// the Out channel and discards it. This is a convenience function to enable
// having a segment at the end of the pipeline handle all results, i.e. having
// no post-pipeline processing.
func (pipeline *Pipeline) AutoDrain() {
	go func() {
		for range pipeline.Out {
		}
		log.Info().Msg("Pipeline closed, auto draining finished.")
	}()
}

// Closes down a Pipeline by closing its In channel and waiting for all
// segments to propagate this close event through the full pipeline,
// terminating all segment goroutines and thus releasing the waitgroup.
// Blocking.
func (pipeline *Pipeline) Close() {
	defer func() {
		recover() // in case In is already closed
		pipeline.wg.Wait()
	}()
	for _, segment := range pipeline.SegmentList {
		segment.Close()
	}
	close(pipeline.In)
}

// Initializes a new Pipeline object and then starts all segment goroutines
// therein. Initialization includes creating any intermediate channels and
// wiring up the segments in the segmentList with them.
func New(segmentList ...segments.Segment) *Pipeline {
	if len(segmentList) == 0 {
		segmentList = []segments.Segment{&pass.Pass{}}
	}
	channels := make([]chan *pb.EnrichedFlow, len(segmentList)+1)
	channels[0] = make(chan *pb.EnrichedFlow)
	for i, segment := range segmentList {
		channels[i+1] = make(chan *pb.EnrichedFlow)
		segment.Rewire(channels[i], channels[i+1])
	}
	return &Pipeline{In: channels[0], Out: channels[len(channels)-1], wg: &sync.WaitGroup{}, SegmentList: segmentList}
}

// Starts the Pipeline by starting all segment goroutines therein.
func (pipeline *Pipeline) Start() {
	for _, segment := range pipeline.SegmentList {
		pipeline.wg.Add(1)
		go segment.Run(pipeline.wg)
	}
}
