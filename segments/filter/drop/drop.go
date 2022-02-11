package drop

import (
	"sync"

	"github.com/bwNetFlow/flowpipeline/segments"
)

type Drop struct {
	segments.BaseFilterSegment
}

func (segment Drop) New(config map[string]string) segments.Segment {
	return &Drop{}
}

func (segment *Drop) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		segment.Drops = nil
		wg.Done()
	}()

	for msg := range segment.In {
		if segment.Drops != nil {
			segment.Drops <- msg
			if r := recover(); r != nil {
				segment.Drops = nil
			}
		}
	}
}

func init() {
	segment := &Drop{}
	segments.RegisterSegment("drop", segment)
}
