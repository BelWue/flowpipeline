package anonymize

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"testing"

	cryptopan "github.com/Yawning/cryptopan"
	"github.com/bwNetFlow/flowpipeline/pb"
	"github.com/bwNetFlow/flowpipeline/segments"
)

// Influx Segment test, passthrough test only
func TestSegment_Influx_passthrough(t *testing.T) {
	result := segments.TestSegment("anonymize", map[string]string{"key": "testkey123jfh789fhj456ezhskila73"},
		&pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 123}, SamplerAddress: []byte{193, 168, 88, 2}})
	if result == nil {
		t.Error("Segment Anonymize is not passing through flows.")
	}
}

// Anonymize Segment benchmark passthrough
func BenchmarkAnonymize(b *testing.B) {
	log.SetOutput(ioutil.Discard)
	os.Stdout, _ = os.Open(os.DevNull)
	var fields = []string{
		"SrcAddr",
		"DstAddr",
		"SamplerAddress",
	}
	anon, _ := cryptopan.New([]byte("testkey123jfh789fhj456ezhskila73"))
	segment := Anonymize{
		EncryptionKey: "testkey123jfh789fhj456ezhskila73",
		anonymizer:    anon,
		Fields:        fields,
	}

	in, out := make(chan *pb.FlowContainer), make(chan *pb.FlowContainer)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &pb.FlowContainer{EnrichedFlow: &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 123}, SamplerAddress: []byte{193, 168, 88, 2}}, Context: context.Background()}
		_ = <-out
	}
	close(in)
}
