// Enriches flows with a geolocation. By default requires RemoteAddr to be set
// as it populates the RemoteCountry field. Optionally matches both addresses
// and writes the results to the Src and DstCountry fields.
package geolocation

import (
	"net"
	"strconv"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/segments"
	maxmind "github.com/oschwald/maxminddb-golang"
)

type GeoLocation struct {
	segments.BaseSegment
	FileName      string // required
	DropUnmatched bool   // optional, default is false, determines whether flows are dropped when location is indeterminate
	MatchBoth     bool   // optional, default is false, determines whether both addresses are matched

	dbHandle *maxmind.Reader
}

func (segment GeoLocation) New(config map[string]string) segments.Segment {
	drop, err := strconv.ParseBool(config["dropunmatched"])
	if err != nil {
		log.Info().Msg("GeoLocation: 'dropunmatched' set to default 'false'.")
	}
	both, err := strconv.ParseBool(config["matchboth"])
	if err != nil {
		log.Info().Msg("GeoLocation: 'matchboth' set to default 'false'.")
	}
	if config["filename"] == "" {
		log.Error().Msg("GeoLocation: This segment requires the 'filename' parameter.")
		return nil
	}
	newSegment := &GeoLocation{
		FileName:      config["filename"],
		DropUnmatched: drop,
		MatchBoth:     both,
	}
	newSegment.dbHandle, err = maxmind.Open(segments.ContainerVolumePrefix + config["filename"])
	if err != nil {
		log.Error().Err(err).Msg("GeoLocation: Could not open specified Maxmind DB file: ")
		return nil
	}
	return newSegment
}

func (segment *GeoLocation) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	defer func() {
		segment.dbHandle.Close()
	}()

	var dbrecord struct {
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
	}

	for msg := range segment.In {
		if !segment.MatchBoth {
			var raddress net.IP
			switch {
			case msg.RemoteAddr == 1: // 1 indicates SrcAddr is the RemoteAddr
				raddress = msg.SrcAddr
			case msg.RemoteAddr == 2: // 2 indicates DstAddr is the RemoteAddr
				raddress = msg.DstAddr
			default:
				if !segment.DropUnmatched {
					segment.Out <- msg
				}
				continue
			}

			err := segment.dbHandle.Lookup(raddress, &dbrecord)
			if err == nil {
				msg.RemoteCountry = dbrecord.Country.ISOCode
			} else {
				log.Error().Err(err).Msg("GeoLocation: Lookup of remote address failed: ")
			}
		} else {
			err := segment.dbHandle.Lookup(msg.SrcAddr, &dbrecord)
			if err == nil {
				msg.SrcCountry = dbrecord.Country.ISOCode
			} else {
				log.Error().Err(err).Msg("GeoLocation: Lookup of source address failed: ")
			}
			err = segment.dbHandle.Lookup(msg.DstAddr, &dbrecord)
			if err == nil {
				msg.DstCountry = dbrecord.Country.ISOCode
			} else {
				log.Error().Err(err).Msg("GeoLocation: Lookup of destination address failed: ")
			}
		}
		segment.Out <- msg
	}
}

func init() {
	segment := &GeoLocation{}
	segments.RegisterSegment("geolocation", segment)
}
