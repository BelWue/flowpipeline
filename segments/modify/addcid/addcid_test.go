package addcid

import (
	"os"
	"sync"
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/rs/zerolog"
)

// AddCid Segment tests are thorough and try every combination
func TestSegment_AddCid_noLocalAddrKeep(t *testing.T) {
	result := segments.TestSegment("addcid", map[string]string{"filename": "../../../examples/enricher/customer_subnets.csv"},
		&pb.EnrichedFlow{RemoteAddr: 0, SrcAddr: []byte{192, 168, 88, 142}})
	if result.Cid != 0 {
		t.Error("([error] Segment AddCid is adding a Cid when the local address is undetermined.")
	}
}

func TestSegment_AddCid_noLocalAddrDrop(t *testing.T) {
	result := segments.TestSegment("addcid", map[string]string{"filename": "../../../examples/enricher/customer_subnets.csv", "dropunmatched": "true"},
		&pb.EnrichedFlow{RemoteAddr: 0, SrcAddr: []byte{192, 168, 88, 142}})
	if result != nil {
		t.Error("([error] Segment AddCid is not dropping the flow as instructed if the local address is undetermined.")
	}
}

func TestSegment_AddCid_localAddrIsDst(t *testing.T) {
	result := segments.TestSegment("addcid", map[string]string{"filename": "../../../examples/enricher/customer_subnets.csv"},
		&pb.EnrichedFlow{RemoteAddr: 1, DstAddr: []byte{192, 168, 88, 42}})
	if result.Cid != 1 {
		t.Error("([error] Segment AddCid is not adding a Cid when the local address is the destination address.")
	}
}

func TestSegment_AddCid_localAddrIsSrc(t *testing.T) {
	result := segments.TestSegment("addcid", map[string]string{"filename": "../../../examples/enricher/customer_subnets.csv"},
		&pb.EnrichedFlow{RemoteAddr: 2, SrcAddr: []byte{192, 168, 88, 142}})
	if result.Cid != 1 {
		t.Error("([error] Segment AddCid is not adding a Cid when the local address is the source address.")
	}
}

func TestSegment_AddCid_bothAddrs(t *testing.T) {
	result := segments.TestSegment("addcid", map[string]string{"matchboth": "1", "filename": "../../../examples/enricher/customer_subnets.csv"},
		&pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}})
	if result.Cid != 1 {
		t.Error("([error] Segment AddCid is not adding a Cid when the local address is the source address.")
	}
}

// AddCid Segment benchmark passthrough
func BenchmarkAddCid(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := AddCid{}.New(map[string]string{"filename": "../../../examples/enricher/customer_subnets.csv"})

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}}
		<-out
	}
	close(in)
}
