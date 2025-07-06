// Replays flows from an sqlite database created by the `sqlite` segment.
package replay

import (
	"github.com/BelWue/flowpipeline/segments"
	"github.com/rs/zerolog/log"
	"sync"
)

type Replay struct {
	segments.BaseSegment

	FileName string
}

func (segment Replay) New(config map[string]string) segments.Segment {
	log.Info().Msg("Replay segment initialized.")
	newsegment := &Replay{}

	if config["filename"] == "" {
		log.Error().Msg("AsLookup: This segment requires a 'filename' parameter.")
		return nil
	}
	return newsegment
}

func (segment *Replay) Run(wg *sync.WaitGroup) {
	panic("unimplemented")
}

func init() {
	segment := &Replay{}
	segments.RegisterSegment("replay", segment)
}
