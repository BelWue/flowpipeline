// Replays flows from an sqlite database created by the `sqlite` segment.
package replay

import (
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/rs/zerolog/log"
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

func ReadFromDB(db *sql.DB, channel chan *pb.EnrichedFlow) {
	rows, err := db.Query("SELECT * FROM flows")
	if err != nil {
		log.Panic().Err(err).Msg("Failed to query flows from database.")
		return
	}
	defer rows.Close()

	for rows.Next() {
		flow := &pb.EnrichedFlow{}

		v := reflect.ValueOf(flow).Elem()
		t := reflect.TypeOf(pb.EnrichedFlow{})

		exportedFields := make([]string, 0, v.NumField())
		for i := 0; i < v.NumField(); i++ {
			if !t.Field(i).IsExported() {
				continue
			}
			exportedFields = append(exportedFields, t.Field(i).Name)
		}

		fieldPointers := make([]any, len(exportedFields))
		var typ, bgpCommunities, asPath, mplsTtl, mplsLabel, mplsIp, layerStack, layerSize, ipv6RoutingHeaderAddresses, srcAddrAnon, dstAddrAnon, samplerAddrAnon, nextHopAnon, validationStatus, normalized, remoteAddr string
		for i, fieldName := range exportedFields {
			switch fieldName {
			case "Type":
				fieldPointers[i] = &typ
			case "BgpCommunities":
				fieldPointers[i] = &bgpCommunities
			case "AsPath":
				fieldPointers[i] = &asPath
			case "MplsTtl":
				fieldPointers[i] = &mplsTtl
			case "MplsLabel":
				fieldPointers[i] = &mplsLabel
			case "MplsIp":
				fieldPointers[i] = &mplsIp
			case "LayerStack":
				fieldPointers[i] = &layerStack
			case "LayerSize":
				fieldPointers[i] = &layerSize
			case "Ipv6RoutingHeaderAddresses":
				fieldPointers[i] = &ipv6RoutingHeaderAddresses
			case "SrcAddrAnon":
				fieldPointers[i] = &srcAddrAnon
			case "DstAddrAnon":
				fieldPointers[i] = &dstAddrAnon
			case "SamplerAddrAnon":
				fieldPointers[i] = &samplerAddrAnon
			case "NextHopAnon":
				fieldPointers[i] = &nextHopAnon
			case "ValidationStatus":
				fieldPointers[i] = &validationStatus
			case "Normalized":
				fieldPointers[i] = &normalized
			case "RemoteAddr":
				fieldPointers[i] = &remoteAddr
			default:
				fieldPointers[i] = v.FieldByName(fieldName).Addr().Interface()
			}
		}

		if err := rows.Scan(fieldPointers...); err != nil {
			log.Error().Err(err).Msg("Failed to scan row from database.")
			continue
		}

		var err error
		flow.Type = pb.EnrichedFlow_FlowType(pb.EnrichedFlow_FlowType_value[typ])
		flow.BgpCommunities, err = ParseUint32Slice(bgpCommunities)
		flow.AsPath, err = ParseUint32Slice(asPath)
		flow.MplsTtl, err = ParseUint32Slice(mplsTtl)
		flow.MplsLabel, err = ParseUint32Slice(mplsLabel)
		flow.MplsIp, err = ParseByteSlices(mplsIp)
		flow.LayerStack, err = ParseLayerStackSlice(layerStack)
		flow.LayerSize, err = ParseUint32Slice(layerSize)
		flow.Ipv6RoutingHeaderAddresses, err = ParseByteSlices(ipv6RoutingHeaderAddresses)
		flow.SrcAddrAnon = pb.EnrichedFlow_AnonymizedType(pb.EnrichedFlow_AnonymizedType_value[srcAddrAnon])
		flow.DstAddrAnon = pb.EnrichedFlow_AnonymizedType(pb.EnrichedFlow_AnonymizedType_value[dstAddrAnon])
		flow.SamplerAddrAnon = pb.EnrichedFlow_AnonymizedType(pb.EnrichedFlow_AnonymizedType_value[samplerAddrAnon])
		flow.NextHopAnon = pb.EnrichedFlow_AnonymizedType(pb.EnrichedFlow_AnonymizedType_value[nextHopAnon])
		flow.ValidationStatus = pb.EnrichedFlow_ValidationStatusType(pb.EnrichedFlow_ValidationStatusType_value[validationStatus])
		flow.Normalized = pb.EnrichedFlow_NormalizedType(pb.EnrichedFlow_NormalizedType_value[normalized])
		flow.RemoteAddr = pb.EnrichedFlow_RemoteAddrType(pb.EnrichedFlow_RemoteAddrType_value[remoteAddr])

		if err != nil {
			log.Error().Err(err).Msg("Failed to parse row data from database.")
			continue
		}

		channel <- flow
	}
}

func init() {
	segment := &Replay{}
	segments.RegisterSegment("replay", segment)
}
