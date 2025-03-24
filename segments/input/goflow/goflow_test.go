package goflow

import (
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
)

// Goflow Segment test, passthrough test only, functionality is tested by Goflow package
func TestSegment_Goflow_passthrough(t *testing.T) {
	result := segments.TestSegment("goflow", map[string]string{"port": "2055"},
		&pb.EnrichedFlow{})
	if result == nil {
		t.Error("([error] Segment Goflow is not passing through flows.")
	}
}
