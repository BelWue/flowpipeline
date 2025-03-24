package flowfilter

import (
	"math/rand"
	"os"
	"sync"
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/rs/zerolog"
)

// FlowFilter Segment testing is basic, the filtering itself is tested in the flowfilter repo
func TestSegment_FlowFilter_accept(t *testing.T) {
	result := segments.TestSegment("flowfilter", map[string]string{"filter": "proto 4"},
		&pb.EnrichedFlow{Proto: 4})
	if result == nil {
		t.Error("([error] Segment FlowFilter dropped a flow incorrectly.")
	}
}

func TestSegment_FlowFilter_deny(t *testing.T) {
	result := segments.TestSegment("flowfilter", map[string]string{"filter": "proto 5"},
		&pb.EnrichedFlow{Proto: 4})
	if result != nil {
		t.Error("([error] Segment FlowFilter accepted a flow incorrectly.")
	}
}

func TestSegment_FlowFilter_syntax(t *testing.T) {
	filter := &FlowFilter{}
	result := filter.New(map[string]string{"filter": "protoo 4"})
	if result != nil {
		t.Error("([error] Segment FlowFilter did something with a syntax error present.")
	}
}

// FlowFilter Segment benchmark passthrough
func BenchmarkFlowFilter(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := FlowFilter{}.New(map[string]string{"filter": "port <50"})

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &pb.EnrichedFlow{SrcPort: uint32(rand.Intn(100))}
		<-out
	}
	close(in)
}
