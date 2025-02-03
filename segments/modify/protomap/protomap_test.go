package protomap

import (
	"io/ioutil"
	"log"
	"os"
	"sync"
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
)

// Protomap Segment test, passthrough test only
func TestSegment_protomap_passthrough(t *testing.T) {
	result := segments.TestSegment("protomap", map[string]string{},
		&pb.EnrichedFlow{})
	if result == nil {
		t.Error("Segment protomap is not passing through flows.")
	}
}

// Protomap Segment test, passthrough test only
func TestSegment_protomap_tcp(t *testing.T) {
	result := segments.TestSegment("protomap", map[string]string{},
		&pb.EnrichedFlow{Proto: 6})
	if result.ProtoName != "TCP" {
		t.Error("Segment protomap is not tagging ProtoName correctly.")
	}
}

// Protomap Segment benchmark passthrough
func BenchmarkProtomap(b *testing.B) {
	log.SetOutput(ioutil.Discard)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := Protomap{}.New(map[string]string{})

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &pb.EnrichedFlow{Proto: 6}
		_ = <-out
	}
	close(in)
}
