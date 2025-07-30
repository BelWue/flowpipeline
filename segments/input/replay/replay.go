// Replays flows from an sqlite database created by the `sqlite` segment.
package replay

import (
	"github.com/BelWue/flowpipeline/segments"
	"github.com/rs/zerolog/log"
	"sync"
)

type Replay struct {
	segments.BaseSegment

	FileName      string
	RespectTiming bool // optional, default is true
}

func (segment Replay) New(config map[string]string) segments.Segment {
	log.Info().Msg("Replay segment initialized.")
	newsegment := &Replay{}

	if config["filename"] == "" {
		log.Error().Msg("AsLookup: This segment requires a 'filename' parameter.")
		return nil
	}
	fileName := config["filename"]

	respectTiming := true
	if config["ignoretiming"] != "" {
		if parsed, err := strconv.ParseBool(config["ignoretiming"]); err == nil {
			respectTiming = parsed
		} else {
			log.Error().Msg("StdIn: Could not parse 'respecttiming' parameter, using default 'true'.")
		}
	} else {
		log.Info().Msg("StdIn: 'respecttiming' set to default 'true'.")
	}

	_, err := sql.Open("sqlite3", fileName)
	if err != nil {
		log.Error().Msgf("Sqlite: Could not open DB file at %s.", fileName)
		return nil
	}

	newsegment.FileName = fileName
	newsegment.RespectTiming = respectTiming

	return newsegment
}

func (segment *Replay) Run(wg *sync.WaitGroup) {
	panic("unimplemented")
}

func init() {
	segment := &Replay{}
	segments.RegisterSegment("replay", segment)
}
