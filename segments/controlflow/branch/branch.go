package branch

import (
	"sync"

	"github.com/bwNetFlow/flowpipeline/segments"
	flow "github.com/bwNetFlow/protobuf/go"
)

// This mirrors the proper implementation in the pipeline package. This
// duplication is to avoid the import cycle.
type Pipeline interface {
	Start()
	Close()
	GetInput() chan *flow.FlowMessage
	GetOutput() <-chan *flow.FlowMessage
	GetDrop() <-chan *flow.FlowMessage
}

type Branch struct {
	segments.BaseSegment
	condition   Pipeline
	then_branch Pipeline
	else_branch Pipeline
}

func (segment Branch) New(config map[string]string) segments.Segment {
	return &Branch{}
}

func (segment *Branch) ImportBranches(condition interface{}, then_branch interface{}, else_branch interface{}) {
	segment.condition = condition.(Pipeline)
	segment.then_branch = then_branch.(Pipeline)
	segment.else_branch = else_branch.(Pipeline)
}

func (segment *Branch) Run(wg *sync.WaitGroup) {
	defer func() {
		segment.condition.Close()
		segment.then_branch.Close()
		segment.else_branch.Close()
		close(segment.Out)
		wg.Done()
	}()

	go segment.condition.Start()
	go segment.then_branch.Start()
	go segment.else_branch.Start()

	go func() { // drain our output
		from_then := segment.then_branch.GetOutput()
		from_else := segment.else_branch.GetOutput()
		for {
			select {
			case msg, ok := <-from_then:
				if !ok {
					from_then = nil
				} else {
					segment.Out <- msg
				}
			case msg, ok := <-from_else:
				if !ok {
					from_else = nil
				} else {
					segment.Out <- msg
				}
			}
			if from_then == nil && from_else == nil {
				return
			}
		}
	}()
	go func() { // move anything from conditional to our two branches
		from_condition_out := segment.condition.GetOutput()
		from_condition_drop := segment.condition.GetDrop()
		for {
			select {
			case msg, ok := <-from_condition_out:
				if !ok {
					from_condition_out = nil
				} else {
					segment.then_branch.GetInput() <- msg
				}
			case msg, ok := <-from_condition_drop:
				if !ok {
					from_condition_drop = nil
				} else {
					segment.else_branch.GetInput() <- msg
				}
			}
			if from_condition_out == nil && from_condition_drop == nil {
				return
			}
		}
	}()
	for msg := range segment.In { // connect our own input to conditional
		segment.condition.GetInput() <- msg
	}
}

func init() {
	segment := &Branch{}
	segments.RegisterSegment("branch", segment)
}
