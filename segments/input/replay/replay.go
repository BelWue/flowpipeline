// Replays flows from an sqlite database created by the `sqlite` segment.
package replay

import (
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/rs/zerolog/log"
)

type Replay struct {
	segments.BaseSegment
	db *sql.DB

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
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	var err error
	segment.db, err = sql.Open("sqlite3", segment.FileName)
	if err != nil {
		log.Panic().Err(err) // Already verified in New()
	}
	defer segment.db.Close()

	flows, err := readFromDB(segment.db)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read flows from database.")
		return
	}

	flowStream := make(chan *pb.EnrichedFlow)
	go replay(flows, segment.RespectTiming, flowStream)

	for {
		select {
		case msg, ok := <-segment.In:
			if !ok {
				return
			}
			segment.Out <- msg
		case row := <-flowStream:
			segment.Out <- row
		}
	}
}

func readFromDB(db *sql.DB) ([]*pb.EnrichedFlow, error) {
	rows, err := db.Query("SELECT * FROM flows")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	flows := make([]*pb.EnrichedFlow, 0)
	for rows.Next() {
		flow := &pb.EnrichedFlow{}

		v := reflect.ValueOf(flow).Elem()
		t := reflect.TypeOf(pb.EnrichedFlow{})

		exportedFields := make([]string, 0, v.NumField())
		for _, field := range reflect.VisibleFields(t) {
			if !field.IsExported() {
				continue
			}
			exportedFields = append(exportedFields, field.Name)
		}

		fieldPointers := make([]any, len(exportedFields))
		var typ, bgpCommunities, asPath, mplsTtl, mplsLabel, mplsIp, layerStack, layerSize, ipv6RoutingHeaderAddresses, srcAddrAnon, dstAddrAnon, samplerAddrAnon, nextHopAnon, validationStatus, normalized, remoteAddr, srcAsPath, dstAsPath string
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
			case "SrcAsPath":
				fieldPointers[i] = &srcAsPath
			case "DstAsPath":
				fieldPointers[i] = &dstAsPath
			default:
				fieldPointers[i] = v.FieldByName(fieldName).Addr().Interface()
			}
		}

		if err := rows.Scan(fieldPointers...); err != nil {
			log.Error().Err(err).Msg("Failed to scan row from database. Probably because the database does only contain a subset of all columns.")
			continue
		}

		var err error
		flow.Type = pb.EnrichedFlow_FlowType(pb.EnrichedFlow_FlowType_value[typ])
		flow.BgpCommunities, err = parseUint32Slice(bgpCommunities)
		flow.AsPath, err = parseUint32Slice(asPath)
		flow.MplsTtl, err = parseUint32Slice(mplsTtl)
		flow.MplsLabel, err = parseUint32Slice(mplsLabel)
		flow.MplsIp, err = parseByteSlices(mplsIp)
		flow.LayerStack, err = parseLayerStackSlice(layerStack)
		flow.LayerSize, err = parseUint32Slice(layerSize)
		flow.Ipv6RoutingHeaderAddresses, err = parseByteSlices(ipv6RoutingHeaderAddresses)
		flow.SrcAddrAnon = pb.EnrichedFlow_AnonymizedType(pb.EnrichedFlow_AnonymizedType_value[srcAddrAnon])
		flow.DstAddrAnon = pb.EnrichedFlow_AnonymizedType(pb.EnrichedFlow_AnonymizedType_value[dstAddrAnon])
		flow.SamplerAddrAnon = pb.EnrichedFlow_AnonymizedType(pb.EnrichedFlow_AnonymizedType_value[samplerAddrAnon])
		flow.NextHopAnon = pb.EnrichedFlow_AnonymizedType(pb.EnrichedFlow_AnonymizedType_value[nextHopAnon])
		flow.ValidationStatus = pb.EnrichedFlow_ValidationStatusType(pb.EnrichedFlow_ValidationStatusType_value[validationStatus])
		flow.Normalized = pb.EnrichedFlow_NormalizedType(pb.EnrichedFlow_NormalizedType_value[normalized])
		flow.RemoteAddr = pb.EnrichedFlow_RemoteAddrType(pb.EnrichedFlow_RemoteAddrType_value[remoteAddr])
		flow.SrcAsPath, err = parseUint32Slice(srcAsPath)
		flow.DstAsPath, err = parseUint32Slice(dstAsPath)

		if err != nil {
			log.Error().Err(err).Msg("Failed to parse row data from database.")
			continue
		}

		flows = append(flows, flow)
	}

	return flows, nil
}

func replay(flows []*pb.EnrichedFlow, respectTiming bool, out chan *pb.EnrichedFlow) {
	if len(flows) == 0 {
		log.Info().Msg("Replay: No flows to replay.")
		return
	}

	for i, flow := range flows {
		if respectTiming && i > 0 {
			previousFlow := flows[i-1]
			prevEndNs := int64(previousFlow.GetTimeFlowEnd())
			currEndNs := int64(flow.GetTimeFlowEnd())
			prevEnd := time.Unix(prevEndNs/1e9, prevEndNs%1e9)
			currEnd := time.Unix(currEndNs/1e9, currEndNs%1e9)
			time.Sleep(currEnd.Sub(prevEnd))
		}
		out <- flow
	}
}

func init() {
	segment := &Replay{}
	segments.RegisterSegment("replay", segment)
}

func parseSlice[T any](s string, elementHandler func(string) (T, error)) ([]T, error) {
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil, fmt.Errorf("invalid format: string does not have surrounding brackets")
	}

	content := s[1 : len(s)-1]           // Get the content inside the brackets.
	entries := strings.Fields(content)   // Split the string by whitespace to get individual number strings.
	result := make([]T, 0, len(entries)) // Create a slice to hold the results

	for _, entry := range entries {
		val, err := elementHandler(entry)
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}

	return result, nil
}

func parseUint32Slice(s string) ([]uint32, error) {
	return parseSlice(s, func(elem string) (uint32, error) {
		val, err := strconv.ParseUint(elem, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("failed to parse number '%s': %w", elem, err)
		}
		return uint32(val), nil
	})
}

func parseByteSlices(s string) ([][]byte, error) {
	return parseSlice(s, func(elemOuter string) ([]byte, error) {
		return parseSlice(elemOuter, func(elemInner string) (byte, error) {
			val, err := strconv.ParseUint(elemInner, 10, 8)
			if err != nil {
				return 0, fmt.Errorf("failed to parse byte value '%s': %w", elemInner, err)
			}
			return byte(val), nil
		})
	})
}

func parseLayerStackSlice(s string) ([]pb.EnrichedFlow_LayerStack, error) {
	return parseSlice(s, func(elem string) (pb.EnrichedFlow_LayerStack, error) {
		return pb.EnrichedFlow_LayerStack(pb.EnrichedFlow_LayerStack_value[elem]), nil
	})
}
