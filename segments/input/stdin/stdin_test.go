package stdin

import (
	"bufio"
	"os"
	"sync"
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/rs/zerolog"
)

// StdIn Segment test, passthrough test only
func TestSegment_StdIn_passthrough(t *testing.T) {
	result := segments.TestSegment("stdin", map[string]string{},
		&pb.EnrichedFlow{})
	if result == nil {
		t.Error("([error] Segment StdIn is not passing through flows.")
	}
}

// Stdin Segment benchmark passthrough
func BenchmarkStdin(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := StdIn{
		scanner: bufio.NewScanner(os.Stdin),
	}

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
