package dropfields

import (
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/rs/zerolog"
)

var (
	testPacketOne = pb.EnrichedFlow{
		SrcAddr: []byte{192, 168, 88, 142},
		DstAddr: []byte{192, 168, 88, 143},
		SrcPort: 1234,
		DstPort: 5678,
		Bytes:   1234567890,
		Packets: 424242,
	}
	testPacketTwo = pb.EnrichedFlow{
		SrcAddr: []byte{0x2a, 0x00, 0x13, 0x98, 0x00, 0x05, 0x8d, 0x01, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x01},
		DstAddr: []byte{0x2a, 0x00, 0x14, 0x50, 0x40, 0x01, 0x08, 0x2b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20, 0x0e},
		SrcPort: 2323,
		DstPort: 4242,
	}
	tests = map[string]struct {
		config   map[string]string
		input    pb.EnrichedFlow
		expected *pb.EnrichedFlow
	}{
		"drop one field": {
			config: map[string]string{"policy": "drop", "fields": "SrcAddr"},
			input:  testPacketOne,
			expected: &pb.EnrichedFlow{
				DstAddr: []byte{192, 168, 88, 143},
				SrcPort: 1234,
				DstPort: 5678,
				Bytes:   1234567890,
				Packets: 424242,
			},
		},
		"keep only SrcAddr": {
			config: map[string]string{"policy": "keep", "fields": "SrcAddr"},
			input:  testPacketOne,
			expected: &pb.EnrichedFlow{
				SrcAddr: []byte{192, 168, 88, 142},
			},
		},
		"keep only DstPort": {
			config: map[string]string{"policy": "keep", "fields": "DstPort"},
			input:  testPacketOne,
			expected: &pb.EnrichedFlow{
				DstPort: 5678,
			},
		},
		"drop three fields": {
			config: map[string]string{"policy": "drop", "fields": "SrcAddr, DstAddr, SrcPort "},
			input:  testPacketOne,
			expected: &pb.EnrichedFlow{
				DstPort: 5678,
				Bytes:   1234567890,
				Packets: 424242,
			},
		},
		"keep two fields": {
			config: map[string]string{"policy": "keep", "fields": " SrcAddr , SrcPort"},
			input:  testPacketTwo,
			expected: &pb.EnrichedFlow{
				SrcAddr: []byte{0x2a, 0x00, 0x13, 0x98, 0x00, 0x05, 0x8d, 0x01, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x01},
				SrcPort: 2323,
			},
		},
	}
)

func TestSegment_DropFields(t *testing.T) {
	for testname, test := range tests {
		//t.Logf("Running test case %s", testname)
		t.Run(testname, func(t *testing.T) {
			result := segments.TestSegment("dropfields", test.config, &test.input)
			if !reflect.DeepEqual(result, test.expected) {
				t.Errorf("[error] Segment DropFields is not returning the proper fields. Got: »%+v« Expected »%+v«", result, test.expected)
			}
		})
	}
}

// DropFields Segment benchmark passthrough
func BenchmarkDropFields(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := DropFields{Policy: PolicyDrop, Fields: []string{"SrcAddr"}}

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &pb.EnrichedFlow{SrcAddr: []byte{192, 168, 88, 142}, DstAddr: []byte{192, 168, 88, 143}}
		<-out
	}
	close(in)
}
