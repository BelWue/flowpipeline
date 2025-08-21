package influx

import (
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
)

// Influx Segment test, passthrough test only
func TestSegment_Influx_passthrough(t *testing.T) {
	result := segments.TestSegment("influx", map[string]string{"org": "testorg", "bucket": "testbucket", "token": "testtoken"},
		&pb.EnrichedFlow{})
	if result == nil {
		t.Error("([error] Segment Influx is not passing through flows.")
	}
}
