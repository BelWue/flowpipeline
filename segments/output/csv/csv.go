// Package csv processes all flows from it's In channel and converts them into
// CSV format. Using it's configuration options it can write to a file or to
// stdout.
package csv

import (
	"encoding/csv"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
)

type Csv struct {
	segments.BaseTextOutputSegment
	writer     *csv.Writer
	fieldNames []string

	Fields string // optional comma-separated list of fields to export, default is "", meaning all fields
}

func (segment Csv) New(config map[string]string) segments.Segment {
	newsegment := &Csv{}
	file, err := segment.GetOutput(config)
	if err != nil {
		log.Error().Err(err).Msg("Csv: File specified in 'filename' is not accessible: ")
		return nil
	}
	log.Info().Msgf("Csv: configured output to %s", file.Name())

	var heading []string
	if config["fields"] != "" {
		protofields := reflect.TypeOf(pb.EnrichedFlow{})
		conffields := strings.Split(config["fields"], ",")
		for _, field := range conffields {
			field = strings.TrimSpace(field)
			protoField, found := protofields.FieldByName(field)
			if !found || !protoField.IsExported() {
				log.Error().Msgf("Csv: Field '%s' specified in 'fields' does not exist.", field)
				return nil
			}
			heading = append(heading, field)
			newsegment.fieldNames = append(newsegment.fieldNames, field)
		}
	} else {
		protofields := reflect.TypeOf(pb.EnrichedFlow{})
		for i := 0; i < protofields.NumField(); i++ {
			field := protofields.Field(i)
			if field.IsExported() {
				newsegment.fieldNames = append(newsegment.fieldNames, field.Name)
				heading = append(heading, field.Name)
			}
		}
		newsegment.Fields = config["fields"]
	}

	newsegment.writer = csv.NewWriter(file)
	if err := newsegment.writer.Write(heading); err != nil {
		log.Error().Err(err).Msg("Csv: Failed to write to destination:")
		return nil
	}
	newsegment.writer.Flush()

	return newsegment
}

func (segment *Csv) Run(wg *sync.WaitGroup) {
	defer func() {
		segment.writer.Flush()
		segment.File.Close()
		close(segment.Out)
		wg.Done()
	}()
	for msg := range segment.In {
		var record []string
		values := reflect.ValueOf(msg).Elem()
		for _, fieldname := range segment.fieldNames {
			value := values.FieldByName(fieldname).Interface()
			switch value := value.(type) {
			case []uint8: // this is necessary for proper formatting
				ipstring := net.IP(value).String()
				if ipstring == "<nil>" {
					ipstring = ""
				}
				record = append(record, ipstring)
			case uint32: // this is because FormatUint is much faster than Sprint
				record = append(record, strconv.FormatUint(uint64(value), 10))
			case uint64: // this is because FormatUint is much faster than Sprint
				record = append(record, strconv.FormatUint(uint64(value), 10))
			case string: // this is because doing nothing is also much faster than Sprint
				record = append(record, value)
			default:
				record = append(record, fmt.Sprint(value))
			}
		}
		segment.writer.Write(record)
		segment.Out <- msg
	}
}

func init() {
	segment := &Csv{}
	segments.RegisterSegment("csv", segment)
}
