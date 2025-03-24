package aslookup

import (
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
)

// TODO: write tests for this
func TestSegment_AsLookup_existingIp(t *testing.T) {
	result := segments.TestSegment("aslookup", map[string]string{"filename": "../../../examples/enricher/lookup.db", "type": "db"},
		&pb.EnrichedFlow{SrcAddr: []byte{192, 168, 1, 10}, DstAddr: []byte{192, 168, 1, 10}})
	if result.SrcAs != 65015 {
		t.Error("([error] Segment AsLookup is not setting the source AS when the corresponding IP exists in the lookup database")
	}
	if result.DstAs != 65015 {
		t.Error("([error] Segment AsLookup is not setting the destination AS when the corresponding IP exists in the lookup database")
	}
}

func TestSegment_AsLookup_nonexistingIp(t *testing.T) {
	result := segments.TestSegment("aslookup", map[string]string{"filename": "../../../examples/enricher/lookup.db", "type": "db"},
		&pb.EnrichedFlow{SrcAddr: []byte{2, 125, 160, 218}, DstAddr: []byte{2, 125, 160, 218}})
	if result.SrcAs != 0 {
		t.Error("([error] Segment AsLookup is setting the source AS when the corresponding IP does not exist in the lookup database.")
	}
	if result.DstAs != 0 {
		t.Error("([error] Segment AsLookup is setting the destination AS when the corresponding IP does not exist in the lookup database.")
	}
}
