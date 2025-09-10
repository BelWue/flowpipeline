package addnetid

import (
	"os"
	"sync"
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/rs/zerolog"
)

// AddNetId Segment tests are thorough and try every combination
// int mode, no remote-address, matchboth false -> Nothing matches
func TestSegment_AddNetId_noLocalAddrKeep(t *testing.T) {
	result := segments.TestSegment("addnetid", map[string]string{"filename": "../../../examples/enricher/subnet_ids.csv", "useintids": "true"},
		&pb.EnrichedFlow{RemoteAddr: 0, SrcAddr: []byte{192, 168, 88, 142}})
	if result.NetId != 0 {
		t.Error("([error] Segment AddNetId is adding a NetId when the local address is undetermined.")
	}
}

// str mode, no remote-address, matchboth false -> Nothing matches
func TestSegment_AddNetIdString_noLocalAddrKeep(t *testing.T) {
	result := segments.TestSegment("addnetid", map[string]string{"filename": "../../../examples/enricher/subnet_id_names.csv", "useintids": "false"},
		&pb.EnrichedFlow{RemoteAddr: 0, SrcAddr: []byte{192, 168, 88, 142}})
	if result.NetIdString != "" {
		t.Error("([error] Segment AddNetId is adding a NetIdString when the local address is undetermined.")
	}
}

// int mode, matchboth true -> Nothing matches
func TestSegment_AddNetId_nobothKeep(t *testing.T) {
	result := segments.TestSegment("addnetid", map[string]string{"matchboth": "1", "filename": "../../../examples/enricher/subnet_ids.csv", "useintids": "true", "dropunmatched": "true"},
		&pb.EnrichedFlow{SrcAddr: []byte{192, 168, 100, 142}})
	if result != nil {
		t.Error("([error] Segment AddNetId is not dropping the flow as instructed if a non matching subnet is passed.")
	}
}

// str mode, matchboth true -> Nothing matches
func TestSegment_AddNetIdString_nobothKeep(t *testing.T) {
	result := segments.TestSegment("addnetid", map[string]string{"matchboth": "1", "filename": "../../../examples/enricher/subnet_id_names.csv", "useintids": "false", "dropunmatched": "true"},
		&pb.EnrichedFlow{SrcAddr: []byte{192, 168, 100, 142}})
	if result != nil {
		t.Error("([error] Segment AddNetId is not dropping the flow as instructed if non matching subnet is passed.")
	}
}

// int mode, no remote-address drop unmatched is true -> No result
func TestSegment_AddNetId_noLocalAddrDrop(t *testing.T) {
	result := segments.TestSegment("addnetid", map[string]string{"filename": "../../../examples/enricher/subnet_ids.csv", "dropunmatched": "true", "useintids": "true"},
		&pb.EnrichedFlow{RemoteAddr: 0, SrcAddr: []byte{192, 168, 88, 142}})
	if result != nil {
		t.Error("([error] Segment AddNetId is not dropping the flow as instructed if the local address is undetermined.")
	}
}

// str mode, no remote-address drop unmatched is true -> No result
func TestSegment_AddNetIdString_noLocalAddrDrop(t *testing.T) {
	result := segments.TestSegment("addnetid", map[string]string{"filename": "../../../examples/enricher/subnet_id_names.csv", "dropunmatched": "true", "useintids": "false"},
		&pb.EnrichedFlow{RemoteAddr: 0, SrcAddr: []byte{192, 168, 88, 142}})
	if result != nil {
		t.Error("([error] Segment AddNetId is not dropping the flow as instructed if the local address is undetermined.")
	}
}

// int mode, dst is remote-address, -> NetID = 1
func TestSegment_AddNetId_localAddrIsDst(t *testing.T) {
	result := segments.TestSegment("addnetid", map[string]string{"filename": "../../../examples/enricher/subnet_ids.csv", "useintids": "true"},
		&pb.EnrichedFlow{RemoteAddr: 1, DstAddr: []byte{192, 168, 88, 42}})
	if result.NetId != 1 {
		t.Error("([error] Segment AddNetId is not adding a NetId when the local address is the destination address.")
	}
}

// str mode, dst is remote-address, -> NetIDString = experiment-1
func TestSegment_AddNetIdString_localAddrIsDst(t *testing.T) {
	result := segments.TestSegment("addnetid", map[string]string{"filename": "../../../examples/enricher/subnet_id_names.csv", "useintids": "false"},
		&pb.EnrichedFlow{RemoteAddr: 1, DstAddr: []byte{192, 168, 88, 42}})
	if result.NetIdString != "experiment-1" {
		t.Error("([error] Segment AddNetId is not adding a NetId when the local address is the destination address.")
	}
}

// int mode, src is remote-address, -> NetID = 1
func TestSegment_AddNetId_localAddrIsSrc(t *testing.T) {
	result := segments.TestSegment("addnetid", map[string]string{"filename": "../../../examples/enricher/subnet_ids.csv", "useintids": "true"},
		&pb.EnrichedFlow{RemoteAddr: 2, SrcAddr: []byte{192, 168, 88, 142}})
	if result.NetId != 1 {
		t.Error("([error] Segment AddNetId is not adding a NetId when the local address is the source address.")
	}
}

// str mode, src is remote-address, -> NetIDString = experiment-1
func TestSegment_AddNetIdString_localAddrIsSrc(t *testing.T) {
	result := segments.TestSegment("addnetid", map[string]string{"filename": "../../../examples/enricher/subnet_id_names.csv", "useintids": "false"},
		&pb.EnrichedFlow{RemoteAddr: 2, SrcAddr: []byte{192, 168, 88, 142}})
	if result.NetIdString != "experiment-1" {
		t.Error("([error] Segment AddNetId is not adding a NetId when the local address is the source address.")
	}
}

// int mode, matchboth, -> SrcId = 1, DstId = 1
func TestSegment_AddNetId_bothAddrs(t *testing.T) {
	result := segments.TestSegment("addnetid", map[string]string{"matchboth": "1", "filename": "../../../examples/enricher/subnet_ids.csv", "useintids": "true"},
		&pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 42}})
	if result.SrcId != 1 {
		t.Error("([error] Segment AddNetId is not adding a SrcId when matching both with a valid source address.")
	}
	if result.DstId != 1 {
		t.Error("([error] Segment AddNetId is not adding a DstId when matching both with a valid destination address.")
	}
}

// str mode, matchboth, -> SrcId = experiment-1, DstId = experiment-1
func TestSegment_AddNetIdString_bothAddrs(t *testing.T) {
	result := segments.TestSegment("addnetid", map[string]string{"matchboth": "1", "filename": "../../../examples/enricher/subnet_id_names.csv", "useintids": "false"},
		&pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 42}})
	if result.SrcIdString != "experiment-1" {
		t.Error("([error] Segment AddNetId is not adding a SrcIdString when matching both with a valid source address.")
	}
	if result.DstIdString != "experiment-1" {
		t.Error("([error] Segment AddNetId is not adding a DstIdString when matching both with a valid destination address.")
	}
}

// int mode, matchboth, -> SrcId = 1, DstId = 0
func TestSegment_AddNetId_bothSrcAddr(t *testing.T) {
	result := segments.TestSegment("addnetid", map[string]string{"matchboth": "1", "filename": "../../../examples/enricher/subnet_ids.csv", "useintids": "true"},
		&pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}})
	if result.DstId != 0 {
		t.Error("([error] Segment AddNetId is adding a DstId when matching both without a valid source address.")
	}
	if result.SrcId != 1 {
		t.Error("([error] Segment AddNetId is not adding a SrcId when matching both with a valid destination address.")
	}
}

// str mode, matchboth, -> SrcId = experiment-1, DstId = ""
func TestSegment_AddNetIdString_bothSrcAddr(t *testing.T) {
	result := segments.TestSegment("addnetid", map[string]string{"matchboth": "1", "filename": "../../../examples/enricher/subnet_id_names.csv", "useintids": "false"},
		&pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 42}})
	if result.DstIdString != "" {
		t.Error("([error] Segment AddNetId is adding a DstIdString when matching both without a valid source address.")
	}
	if result.SrcIdString != "experiment-1" {
		t.Error("([error] Segment AddNetId is not adding a SrcIdString when matching both with a valid destination address.")
	}
}

// int mode, matchboth, -> SrcId = 0, DstId = 1
func TestSegment_AddNetId_bothDstAddr(t *testing.T) {
	result := segments.TestSegment("addnetid", map[string]string{"matchboth": "1", "filename": "../../../examples/enricher/subnet_ids.csv", "useintids": "true"},
		&pb.EnrichedFlow{DstAddr: []byte{192, 168, 88, 42}})
	if result.SrcId != 0 {
		t.Error("([error] Segment AddNetId is adding a SrcId when matching both without a valid source address.")
	}
	if result.DstId != 1 {
		t.Error("([error] Segment AddNetId is not adding a DstId when matching both with a valid destination address.")
	}
}

// str mode, matchboth, -> SrcId = "", DstId = experiment-1
func TestSegment_AddNetIdString_bothDstAddr(t *testing.T) {
	result := segments.TestSegment("addnetid", map[string]string{"matchboth": "1", "filename": "../../../examples/enricher/subnet_id_names.csv", "useintids": "false"},
		&pb.EnrichedFlow{DstAddr: []byte{192, 168, 88, 42}})
	if result.SrcIdString != "" {
		t.Error("([error] Segment AddNetId is adding a SrcIdString when matching both without a valid source address.")
	}
	if result.DstIdString != "experiment-1" {
		t.Error("([error] Segment AddNetId is not adding a DstIdString when matching both with a valid destination address.")
	}
}

// AddNetId Segment benchmark passthrough
func BenchmarkAddCNetId(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := AddNetId{}.New(map[string]string{"filename": "../../../examples/enricher/subnet_id_names.csv", "useintids": "true"})

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
