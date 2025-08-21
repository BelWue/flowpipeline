// The `count` segment counts flows passing it. This is mainly for debugging
// flowpipelines. For instance, placing two of these segments around a
// `flowfilter` segment allows users to use the `prefix` parameter with values
// `"pre-filter: "`  and `"post-filter: "` to obtain a count of flows making it
// through the filter without resorting to some command employing `| wc -l`.
//
// The result is printed upon termination of the flowpipeline or to a file if a
// filename is configured.
package count

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/segments"
)

type Count struct {
	segments.BaseTextOutputSegment
	count  uint64
	Prefix string // optional, default is empty, a string which is printed along with the result
}

func (segment Count) New(config map[string]string) segments.Segment {
	file, err := segment.GetOutput(config)
	if err != nil {
		log.Error().Err(err).Msg("Count: File specified in 'filename' is not accessible: ")
		return nil
	}
	log.Info().Msgf("Count: configured output to %s", file.Name())
	return &Count{
		Prefix: config["prefix"],
		BaseTextOutputSegment: segments.BaseTextOutputSegment{
			File: file,
		},
	}
}

func (segment *Count) Run(wg *sync.WaitGroup) {
	defer func() {
		segment.File.Close()
		close(segment.Out)
		wg.Done()
	}()
	for msg := range segment.In {
		segment.count += 1
		segment.Out <- msg
	}

	out := fmt.Sprintf("%s%d", segment.Prefix, segment.count)
	segment.File.WriteString(out)
}

func init() {
	segment := &Count{}
	segments.RegisterSegment("count", segment)
}
