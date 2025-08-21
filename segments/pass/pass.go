// The `pass` segment serves as a heavily annotated template for new segments. So
// does this piece of documentation. Aside from summarizing what a segment does,
// it should include a description of all the parameters it accepts as well as any
// caveats users should be aware of.
package pass

import (
	"sync"

	"github.com/BelWue/flowpipeline/segments"
)

// The Pass Segment is considered a template for any additional Segments, as it
// showcases the exact implementation.
type Pass struct {
	segments.BaseSegment // always embed this, no need to repeat I/O chan code
	// add any additional fields here
}

// Every Segment must implement a New method, even if there isn't any config
// it is interested in.
func (segment Pass) New(config map[string]string) segments.Segment {
	// do config stuff here, add it to fields maybe
	return &Pass{}
}

// The main goroutine of any Segment. Any Run method must:
// 1. close(segment.Out) when the In channel is closed by the previous segment or the Pipeline itself
// 2. call wg.Done() before exiting
// 3. if exiting for any other reason, use segment.ShutdownParentPipeline() or just continue to pass from In to Out
//
// Usually, when using a range over In in combination with below defer, nothing
// will go wrong. However, some segments have a legitimate use case for using
// `for {}`, in which case care must be taken to keep draining In.
func (segment *Pass) Run(wg *sync.WaitGroup) {
	defer func() {
		// This defer clause is important and needs to be present in
		// any Segment.Run method in some form, but with at least the
		// following two statements.
		close(segment.Out)
		wg.Done()
	}()
	for msg := range segment.In {
		// Work with the flow messages here.
		segment.Out <- msg
	}
}

// Every Segment needs an init() function of some form in its file to be
// callable from config. An unregistered Segment will only be available using
// the API.
func init() {
	segment := &Pass{}
	segments.RegisterSegment("pass", segment)
}
