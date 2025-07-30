// Drops fields from any passing flow.
package dropfields

import (
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
)

type Policy int

const (
	PolicyDrop Policy = iota
	PolicyKeep
)

var (
	FieldSplitRegex = regexp.MustCompile(`[\s,;:]+`)
)

type DropFields struct {
	segments.BaseSegment
	Policy Policy   // required, determines whether to keep or drop fields
	Fields []string // required, determines which fields are kept/dropped
}

func (segment *DropFields) New(config map[string]string) segments.Segment {
	var (
		policy Policy
		fields []string
	)

	// parse policy
	switch config["policy"] {
	case "keep":
		policy = PolicyKeep
	case "drop":
		policy = PolicyDrop
	default:
		log.Fatal().Msg("DropFields: The 'policy' parameter is required to be either 'keep' or 'drop'.")
	}

	// parse fields
	fields = FieldSplitRegex.Split(strings.TrimSpace(config["fields"]), -1)
	if len(fields) == 0 {
		log.Warn().Msg("DropFields: The 'fields' parameter can not be empty.")
	}

	return &DropFields{
		Policy: policy,
		Fields: fields,
	}
}

func (segment *DropFields) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()
	for original := range segment.In {
		// get reflected value of original flow
		reflectedOriginal := reflect.ValueOf(original).Elem()
		switch segment.Policy {
		case PolicyKeep:
			resultFlow := &pb.EnrichedFlow{}
			for _, fieldName := range segment.Fields {
				originalField := reflectedOriginal.FieldByName(fieldName)
				resultFlowDestinationField := reflect.ValueOf(resultFlow).Elem().FieldByName(fieldName)
				if originalField.IsValid() && resultFlowDestinationField.CanSet() {
					resultFlowDestinationField.Set(originalField)
				} else {
					log.Fatal().Msgf("DropFields: Field '%s' is not valid or can not be set.", fieldName)
				}
			}
			segment.Out <- resultFlow
		case PolicyDrop:
			for _, fieldName := range segment.Fields {
				originalField := reflect.Indirect(reflectedOriginal).FieldByName(fieldName)
				if originalField.IsValid() && originalField.CanSet() {
					originalField.SetZero()
				}
			}
			segment.Out <- original
		}
	}
}

func init() {
	segment := &DropFields{}
	segments.RegisterSegment("dropfields", segment)
}
