// The `count` segment counts flows passing it. This is mainly for debugging
// flowpipelines. For instance, placing two of these segments around a
// `flowfilter` segment allows users to use the `prefix` parameter with values
// `"pre-filter: "`  and `"post-filter: "` to obtain a count of flows making it
// through the filter without resorting to some command employing `| wc -l`.
//
// The result is printed upon termination of the flowpipeline.
package count

import (
	"os"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/segments"
)

type Count struct {
	segments.BaseSegment
	count  uint64
	Prefix string // optional, default is empty, a string which is printed along with the result
}

func (segment Count) New(config map[string]string) segments.Segment {
	return &Count{
		Prefix: config["prefix"],
	}
}

func (segment *Count) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()
	for msg := range segment.In {
		segment.count += 1
		segment.Out <- msg
	}
	// use custom log to print to stderr without any filtering
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	logger.Level(zerolog.DebugLevel)
	logger.Info().Msgf("%s%d", segment.Prefix, segment.count)
}

func init() {
	segment := &Count{}
	segments.RegisterSegment("count", segment)
}
