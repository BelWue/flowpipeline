package remoteaddress

import (
	"os"
	"sync"
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/rs/zerolog"
)

// RemoteAddress Segment testing is basically checking whether switch/case is working okay...
func TestSegment_RemoteAddress(t *testing.T) {
	result := segments.TestSegment("remoteaddress", map[string]string{"policy": "border"},
		&pb.EnrichedFlow{FlowDirection: 0})
	if result.RemoteAddr != 1 {
		t.Error("([error] Segment RemoteAddress is not determining RemoteAddr correctly.")
	}
}

func TestSegment_RemoteAddress_localAddrIsDst(t *testing.T) {
	result := segments.TestSegment("remoteaddress", map[string]string{"policy": "cidr", "filename": "../../../examples/enricher/customer_subnets.csv"},
		&pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 42}})
	if result.RemoteAddr != 1 {
		t.Error("([error] Segment RemoteAddress is not determining the local address correctly by 'cidr'.")
	}
}

// RemoteAddress Segment benchmark passthrough
func BenchmarkRemoteAddress(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := RemoteAddress{}.New(map[string]string{"policy": "cidr", "filename": "../../../examples/enricher/customer_subnets.csv"})

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 42}}
		<-out
	}
	close(in)
}
