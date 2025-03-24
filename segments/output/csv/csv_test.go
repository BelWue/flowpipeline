package csv

import (
	"net"
	"os"
	"sync"
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/rs/zerolog"
)

// Csv Segment test, passthrough test
func TestSegment_Csv_passthrough(t *testing.T) {
	result := segments.TestSegment("csv", map[string]string{},
		&pb.EnrichedFlow{Type: 3, SamplerAddress: net.ParseIP("192.0.2.1")})

	if result.Type != 3 {
		t.Error("([error] Segment Csv is not working.")
	}
}

// Csv Segment benchmark passthrough
func BenchmarkCsv(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := Csv{}.New(map[string]string{})

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &pb.EnrichedFlow{Proto: 45}
		<-out
	}
	close(in)
}
