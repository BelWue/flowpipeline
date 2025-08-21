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
	result := segments.TestSegment("remoteaddress", map[string]string{"policy": "cidr", "filename": "../../../examples/configuration/enricher/subnet_ids.csv"},
		&pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 42}})
	if result.RemoteAddr != 1 {
		t.Error("([error] Segment RemoteAddress is not determining the remote address correctly by 'cidr'.")
	}
}

func TestSegment_RemoteAddress_localAddrIsSrc(t *testing.T) {
	result := segments.TestSegment("remoteaddress", map[string]string{"policy": "cidr", "filename": "../../../examples/enricher/subnet_ids.csv"},
		&pb.EnrichedFlow{DstAddr: []byte{192, 168, 88, 42}})
	if result.RemoteAddr != 2 {
		t.Error("([error] Segment RemoteAddress is not determining the remote address correctly by 'cidr'.")
	}
}

// if both are matching src is determined as the remote address
func TestSegment_RemoteAddress_localAddrIs(t *testing.T) {
	result := segments.TestSegment("remoteaddress", map[string]string{"policy": "cidr", "filename": "../../../examples/enricher/subnet_ids.csv"},
		&pb.EnrichedFlow{DstAddr: []byte{192, 168, 88, 42}, SrcAddr: []byte{192, 168, 88, 43}})
	if result.RemoteAddr != 1 {
		t.Error("([error] Segment RemoteAddress is not determining the remote address correctly by 'cidr'.")
	}
}

// RemoteAddress Segment benchmark passthrough
func BenchmarkRemoteAddress(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := RemoteAddress{}.New(map[string]string{"policy": "cidr", "filename": "../../../examples/configuration/enricher/subnet_ids.csv"})

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
