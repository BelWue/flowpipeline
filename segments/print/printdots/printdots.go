// Prints a dot every n flows.
package printdots

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/segments"
)

type PrintDots struct {
	segments.BaseSegment
	FlowsPerDot uint64 // optional, default is 5000
}

func (segment PrintDots) New(config map[string]string) segments.Segment {
	var fpd uint64 = 5000
	if parsedFpd, err := strconv.ParseUint(config["flowsperdot"], 10, 32); err == nil {
		fpd = parsedFpd
	} else {
		if config["flowsperdot"] != "" {
			log.Error().Msg("PrintDots: Could not parse 'flowsperdot' parameter, using default 5000.")
		} else {
			log.Info().Msg("PrintDots: 'flowsperdot' set to default 5000.")
		}
	}
	return &PrintDots{
		FlowsPerDot: fpd,
	}
}

func (segment *PrintDots) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()
	count := uint64(0)
	for msg := range segment.In {
		if count += 1; count >= segment.FlowsPerDot {
			fmt.Printf(".")
			count = 0
		}
		segment.Out <- msg
	}
}

func init() {
	segment := &PrintDots{}
	segments.RegisterSegment("printdots", segment)
}
