package printflowdump

import (
	"io"
	"log"
	"os"
	"sync"
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
)

// PrintFlowdump Segment test, passthrough test only
func TestSegment_PrintFlowdump_passthrough(t *testing.T) {
	result := segments.TestSegment("printflowdump", map[string]string{},
		&pb.EnrichedFlow{})
	if result == nil {
		t.Error("Segment PrintFlowDump is not passing through flows.")
	}
}

// PrintFlowdump Segment benchmark passthrough
func BenchmarkPrintFlowdump(b *testing.B) {
	log.SetOutput(io.Discard)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := PrintFlowdump{}

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &pb.EnrichedFlow{}
		<-out
	}
	close(in)
}
