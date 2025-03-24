package geolocation

import (
	"os"
	"sync"
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/rs/zerolog"
)

// GeoLocation Segment tests are thorough and try every combination
func TestSegment_GeoLocation_noRemoteAddrKeep(t *testing.T) {
	result := segments.TestSegment("geolocation", map[string]string{"filename": "../../../examples/enricher/GeoLite2-Country-Test.mmdb"},
		&pb.EnrichedFlow{RemoteAddr: 0, SrcAddr: []byte{2, 125, 160, 218}, DstAddr: []byte{2, 125, 160, 218}})
	if result.RemoteCountry != "" {
		t.Error("([error] Segment GeoLocation is adding a RemoteCountry when the remote address is undetermined.")
	}
}

func TestSegment_GeoLocation_noRemoteAddrDrop(t *testing.T) {
	result := segments.TestSegment("geolocation", map[string]string{"filename": "../../../examples/enricher/GeoLite2-Country-Test.mmdb", "dropunmatched": "1"},
		&pb.EnrichedFlow{RemoteAddr: 0, SrcAddr: []byte{2, 125, 160, 218}, DstAddr: []byte{2, 125, 160, 218}})
	if result != nil {
		t.Error("([error] Segment GeoLocation is not dropping the flow as instructed if the remote address is undetermined.")
	}
}

func TestSegment_GeoLocation_remoteAddrIsSrc(t *testing.T) {
	result := segments.TestSegment("geolocation", map[string]string{"filename": "../../../examples/enricher/GeoLite2-Country-Test.mmdb"},
		&pb.EnrichedFlow{RemoteAddr: 1, SrcAddr: []byte{2, 125, 160, 218}})
	if result.RemoteCountry != "GB" {
		t.Error("([error] Segment GeoLocation is not adding RemoteCountry when the remote address is the source address.")
	}
}

func TestSegment_GeoLocation_remoteAddrIsDst(t *testing.T) {
	result := segments.TestSegment("geolocation", map[string]string{"filename": "../../../examples/enricher/GeoLite2-Country-Test.mmdb"},
		&pb.EnrichedFlow{RemoteAddr: 2, DstAddr: []byte{2, 125, 160, 218}})
	if result == nil || result.RemoteCountry != "GB" {
		t.Error("([error] Segment GeoLocation is not adding RemoteCountry when the remote address is the destination address.")
	}
}

func TestSegment_GeoLocation_both(t *testing.T) {
	result := segments.TestSegment("geolocation", map[string]string{"filename": "../../../examples/enricher/GeoLite2-Country-Test.mmdb", "matchboth": "1"},
		&pb.EnrichedFlow{DstAddr: []byte{2, 125, 160, 218}})
	if result == nil || result.DstCountry != "GB" {
		t.Error("([error] Segment GeoLocation is not adding DstCountry correctly.")
	}
}

// GeoLocation Segment benchmark passthrough
func BenchmarkGeoLocation(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := GeoLocation{}.New(map[string]string{"filename": "../../../examples/enricher/GeoLite2-Country-Test.mmdb"})

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &pb.EnrichedFlow{RemoteAddr: 2, DstAddr: []byte{2, 125, 160, 218}}
		<-out
	}
	close(in)
}
