package json

import (
	"os"
	"sync"
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/rs/zerolog"
)

// Json Segment test, passthrough test only
func TestSegment_Json_passthrough(t *testing.T) {
	result := segments.TestSegment("json", map[string]string{},
		&pb.EnrichedFlow{})
	if result == nil {
		t.Error("([error] Segment Json is not passing through flows.")
	}
}

// Json Segment benchmark passthrough
func BenchmarkJson(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := Json{}.New(map[string]string{})

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
