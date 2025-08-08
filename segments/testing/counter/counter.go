package counter

import (
	"sync"

	"github.com/BelWue/flowpipeline/segments"
)

type Counter struct {
	segments.BaseSegment
	Counter int
}

func (segment Counter) New(config map[string]string) segments.Segment {
	return &Counter{
		Counter: 0,
	}
}

func (segment *Counter) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	for msg := range segment.In {
		segment.Counter += 1
		segment.Out <- msg
	}
}

func init() {
	segment := &Counter{}
	segments.RegisterSegment("counter", segment)
}
