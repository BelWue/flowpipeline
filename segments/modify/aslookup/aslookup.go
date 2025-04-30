package aslookup

import (
	"net"
	"os"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/segments"
	"github.com/banviktor/asnlookup/pkg/database"
)

type AsLookup struct {
	segments.BaseSegment
	FileName string
	Type     string

	asDatabase database.Database
}

func (segment AsLookup) New(config map[string]string) segments.Segment {

	newSegment := &AsLookup{}

	// parse options
	if config["filename"] == "" {
		log.Error().Msg("AsLookup: This segment requires a 'filename' parameter.")
		return nil
	}
	newSegment.FileName = config["filename"]

	if config["type"] == "db" {
		newSegment.Type = "db"
	} else if config["type"] == "mrt" {
		newSegment.Type = "mrt"
	} else {
		log.Info().Msg("AsLookup: 'type' set to default 'db'.")
		newSegment.Type = "db"
	}

	// open lookup file
	lookupfile, err := os.OpenFile(config["filename"], os.O_RDONLY, 0)
	if err != nil {
		log.Error().Err(err).Msg("AsLookup: Error opening lookup file: ")
		return nil
	}
	defer lookupfile.Close()

	// lookup file can either be an MRT file or a lookup database generated with asnlookup
	// see: https://github.com/banviktor/asnlookup
	if newSegment.Type == "db" {
		// open lookup db
		db, err := database.NewFromDump(lookupfile)
		if err != nil {
			log.Error().Err(err).Msg("AsLookup: Error parsing database file: ")
		}
		newSegment.asDatabase = db
	} else {
		// parse with asnlookup
		builder := database.NewBuilder()
		if err = builder.ImportMRT(lookupfile); err != nil {
			log.Error().Err(err).Msg("AsLookup: Error parsing MRT file: ")
		}

		// build lookup database
		db, err := builder.Build()
		if err != nil {
			log.Error().Err(err).Msg("AsLookup: Error building lookup database: ")
		}
		newSegment.asDatabase = db
	}

	return newSegment
}

func (segment *AsLookup) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()
	for msg := range segment.In {
		// Look up destination AS
		dstIp := net.ParseIP(msg.DstAddrObj().String())
		dstAs, err := segment.asDatabase.Lookup(dstIp)
		if err != nil {
			log.Warn().Err(err).Msgf("AsLookup: Failed to look up ASN for %s", msg.DstAddrObj().String())
			segment.Out <- msg
			continue
		}
		msg.DstAs = dstAs.Number

		// Look up source AS
		srcIp := net.ParseIP(msg.SrcAddrObj().String())
		srcAs, err := segment.asDatabase.Lookup(srcIp)
		if err != nil {
			log.Warn().Err(err).Msgf("AsLookup: Failed to look up ASN for %s", msg.SrcAddrObj().String())
			segment.Out <- msg
			continue
		}
		msg.SrcAs = srcAs.Number

		segment.Out <- msg
	}
}

func init() {
	segment := &AsLookup{}
	segments.RegisterSegment("aslookup", segment)
}
