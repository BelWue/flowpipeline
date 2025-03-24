// Counts the number of passing flows and prints the result on termination.
// Typically used to test flow counts before and after a filter segment, best
// used with `prefix: pre` and `prefix: post`.
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
