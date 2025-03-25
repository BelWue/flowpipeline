package count

import (
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
)

// Count Segment test, passthrough test only
func TestSegment_Count_passthrough(t *testing.T) {
	result := segments.TestSegment("count", map[string]string{"prefix": "Test: "},
		&pb.EnrichedFlow{})
	if result == nil {
		t.Error("([error] Segment Count is not passing through flows.")
	}
}
