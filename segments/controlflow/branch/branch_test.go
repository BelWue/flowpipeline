package branch

import (
	"sync"
	"testing"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
)

// Branch Segment test, passthrough test
// This does not work currently, as segment tests are scoped for segment
// package only, and this specific segment requires some pipeline
// initialization, which would lead to an import cycle. Thus, this test
// confirms that it fails silently, and this segment is instead tested from the
// pipeline package test files.
func TestSegment_Branch_passthrough(t *testing.T) {
	segment := segments.LookupSegment("branch").New(map[string]string{}).(*Branch)
	if segment == nil {
		log.Fatal().Msg("Configured segment 'branch' could not be initialized properly, see previous messages.")
	}
	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	// this would timeout if it worked properly, instead it logs an error and returns
	segment.Run(wg)
}
