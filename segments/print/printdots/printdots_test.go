package printdots

import (
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
)

// PrintDots Segment test, passthrough test only
func TestSegment_PrintDots_passthrough(t *testing.T) {
	result := segments.TestSegment("printdots", map[string]string{"flowsPerDot": "100"},
		&pb.EnrichedFlow{})
	if result == nil {
		t.Error("([error] Segment PrintDots is not passing through flows.")
	}
}

func TestSegment_PrintDots_instanciation(t *testing.T) {
	printDots := &PrintDots{}
	result := printDots.New(map[string]string{})
	if result == nil {
		t.Error("([error] Segment PrintDots did not intiate despite good base config.")
	}

	printDots = &PrintDots{}
	result = printDots.New(map[string]string{"flowsperdot": "twelve"})
	if result == nil {
		t.Error("([error] Segment PrintDots did not fallback from bad base config.")
	}

	printDots = &PrintDots{}
	result = printDots.New(map[string]string{"flowsperdot": "12"})
	if result == nil {
		t.Error("([error] Segment PrintDots did not intiate despite good base config.")
	}
}
