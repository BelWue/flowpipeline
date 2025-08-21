// The `branch` segment is used to select the further progression of the pipeline
// between to branches. To this end, it uses additional syntax that other segments
// do not have access to, namely the `if`, `then` and `else` keys which can
// contain lists of segments that constitute embedded pipelines.
//
// The any of these three keys may be empty and they are by default. The `if`
// segments receive the flows entering the `branch` segment unconditionally. If
// the segments in `if` proceed any flow from the input all the way to the end of
// the `if` segments, this flow will be moved on to the `then` segments. If flows
// are dropped at any point within the `if` segments, they will be moved on to the
// `else` branch immediately, shortcutting the traversal of the `if` segments. Any
// edits made to flows during the `if` segments will be persisted in either
// branch, `then` and `else`, as well as after the flows passed from the `branch`
// segment into consecutive segments. Dropping flows behaves regularly in both
// branches, but note that flows can not be dropped within the `if` branch
// segments, as this is taken as a cue to move them into the `else` branch.
// The `bypass-messages` flag can be used to forward all incoming messages to the
// next segment, regardles of them being droped or forwarded inside the `if` or `else`
// branch.
//
// If any of these three lists of segments (or subpipelines) is empty, the
// `branch` segment will behave as if this subpipeline consisted of a single
// `pass` segment.
//
// Instead of a minimal example, the following more elaborate one highlights all
// TCP flows while printing to standard output and keeps only these highlighted
// ones in a sqlite export:
//
// ```yaml
// - segment: branch
//   if:
//   - segment: flowfilter
//     config:
//       filter: proto tcp
//   - segment: elephant
//   then:
//   - segment: printflowdump
//     config:
//       highlight: 1
//   else:
//   - segment: printflowdump
//   - segment: drop
//
// - segment: sqlite
//   config:
//     filename: tcponly.sqlite
// ```
package branch

import (
	"strconv"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
)

// This mirrors the proper implementation in the pipeline package. This
// duplication is to avoid the import cycle.
type Pipeline interface {
	Start()
	Close()
	GetInput() chan *pb.EnrichedFlow
	GetOutput() <-chan *pb.EnrichedFlow
	GetDrop() <-chan *pb.EnrichedFlow
}

type Branch struct {
	segments.BaseFilterSegment
	condition      Pipeline
	then_branch    Pipeline
	else_branch    Pipeline
	bypassMessages bool //optional, default is false, forward all ingoing messages to the next segment (ignoring filtering of the branch segments)
}

func (segment Branch) New(config map[string]string) segments.Segment {
	bypassMessages := false
	if config["bypass-messages"] != "" {
		b, err := strconv.ParseBool(config["bypass-messages"])
		if err != nil {
			log.Fatal().Err(err).Msg("Branch: Failed to parse bypass-messages config option")
		}
		bypassMessages = b
	}
	return &Branch{bypassMessages: bypassMessages}
}

func (segment *Branch) ImportBranches(condition interface{}, then_branch interface{}, else_branch interface{}) {
	segment.condition = condition.(Pipeline)
	segment.then_branch = then_branch.(Pipeline)
	segment.else_branch = else_branch.(Pipeline)
}

func (segment *Branch) Run(wg *sync.WaitGroup) {
	if segment.condition == nil || segment.then_branch == nil || segment.else_branch == nil {
		log.Error().Msg("Branch: Uninitialized branches. This is expected during standalone testing of this package. The actual test is done as part of the pipeline package, as this segment embeds further pipelines.")
		return
	}
	defer func() {
		segment.condition.Close()
		segment.then_branch.Close()
		segment.else_branch.Close()
		close(segment.Out)
		if segment.Drops != nil {
			close(segment.Drops)
		}
		wg.Done()
	}()

	go segment.condition.Start()
	go segment.then_branch.Start()
	go segment.else_branch.Start()

	go drainOutput(segment)
	go forwardBasedOnCondition(segment)

	for msg := range segment.In { // connect our own input to conditional
		segment.condition.GetInput() <- msg
	}
}

func forwardBasedOnCondition(segment *Branch) {
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
}

func drainOutput(segment *Branch) {
	from_then := segment.then_branch.GetOutput()
	from_else := segment.else_branch.GetOutput()
	from_then_drop := segment.then_branch.GetDrop()
	from_else_drop := segment.else_branch.GetDrop()
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

		case msg, ok := <-from_then_drop:
			if !ok {
				from_then_drop = nil
			} else {
				if segment.bypassMessages {
					segment.Out <- msg
				} else if segment.Drops != nil {
					segment.Drops <- msg
				}
			}

		case msg, ok := <-from_else_drop:
			if !ok {
				from_else_drop = nil
			} else {
				if segment.bypassMessages {
					segment.Out <- msg
				} else if segment.Drops != nil {
					segment.Drops <- msg
				}
			}
		}
		if from_then == nil || from_else == nil || from_then_drop == nil || from_else_drop == nil {
			return
		}
	}
}

func init() {
	segment := &Branch{}
	segments.RegisterSegment("branch", segment)
}
