// The segment `sync_timestamps` tries to fill empty time fields using existing ones.
// It works on the following fields:
// - TimeFlowStart:
//   - TimeFlowStart
//   - TimeFlowStartMs
//   - TimeFlowStartNs
// - TimeFlowEnd:
//   - TimeFlowEnd
//   - TimeFlowEndMs
//   - TimeFlowEndNs
// - TimeReceived:
//   - TimeReceived
//   - TimeReceivedNs
package sync_timestamps

import (
	"sync"

	"github.com/BelWue/flowpipeline/segments"
)

type SyncTimestamps struct {
	segments.BaseSegment
}

func (segment SyncTimestamps) New(config map[string]string) segments.Segment {
	return &SyncTimestamps{}
}

func (segment *SyncTimestamps) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	for msg := range segment.In {
		msg.SyncMissingTimeStamps()
		segment.Out <- msg
	}
}

func init() {
	segment := &SyncTimestamps{}
	segments.RegisterSegment("sync_timestamps", segment)
}
