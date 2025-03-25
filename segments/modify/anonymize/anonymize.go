// Anonymize uses the CryptoPan prefix-preserving IP address sanitization as specified by J. Fan, J. Xu, M. Ammar, and S. Moon.
// By default this segment is anonymizing the SrcAddr, DstAddr and SamplerAddress fields in the flowmessage.
// The required encryption key has to be created with the length of 32 chars.
package anonymize

import (
	"strings"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/segments"
	cryptopan "github.com/Yawning/cryptopan"
)

type Anonymize struct {
	segments.BaseSegment
	EncryptionKey string   // required, key for anonymization by cryptopan
	Fields        []string // optional, list of Fields to anonymize their IP address. Default if nmot set are all available fields: SrcAddr, DstAddr, SamplerAddress

	anonymizer *cryptopan.Cryptopan
}

func (segments Anonymize) New(config map[string]string) segments.Segment {
	var encryptionKey string
	if config["key"] == "" {
		log.Error().Msg("Anonymize: Missing configuration parameter 'key'. Please set the key to use for anonymization of IP addresses.")
		return nil
	} else {
		encryptionKey = config["key"]
	}

	// set default fields with IPs to anonymize
	var fields = []string{
		"SrcAddr",
		"DstAddr",
		"SamplerAddress",
	}
	if config["fields"] == "" {
		log.Info().Msgf("Anonymize: Missing configuration parameter 'fields'. Using default fields '%s' to anonymize.", fields)
	} else {
		fields = []string{}
		for _, field := range strings.Split(config["fields"], ",") {
			log.Info().Msgf("Anonymize: custom field found: %s", field)
			fields = append(fields, field)
		}
	}

	ekb := []byte(encryptionKey)
	anon, err := cryptopan.New(ekb)
	if err != nil {
		if _, ok := err.(cryptopan.KeySizeError); ok {
			log.Error().Msgf("Anonymize: Key has insufficient length %d, please specifiy one with more than 32 chars.", len(encryptionKey))
		} else {
			log.Error().Msgf("Anonymize: error creating anonymizer: %e", err)
		}
		return nil
	}

	return &Anonymize{
		EncryptionKey: encryptionKey,
		anonymizer:    anon,
		Fields:        fields,
	}
}

func (segment *Anonymize) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	for msg := range segment.In {
		for _, field := range segment.Fields {
			switch field {
			case "SrcAddr":
				if msg.SrcAddrObj() == nil {
					continue
				}
				msg.SrcAddr = segment.anonymizer.Anonymize(msg.SrcAddr)
				msg.SrcAddrAnon = 1
			case "DstAddr":
				if msg.DstAddrObj() == nil {
					continue
				}
				msg.DstAddr = segment.anonymizer.Anonymize(msg.DstAddr)
				msg.DstAddrAnon = 1
			case "SamplerAddress":
				if msg.SamplerAddressObj() == nil {
					continue
				}
				msg.SamplerAddress = segment.anonymizer.Anonymize(msg.SamplerAddress)
				msg.SamplerAddrAnon = 1
			case "NextHop":
				if msg.NextHopObj() == nil {
					continue
				}
				msg.NextHop = segment.anonymizer.Anonymize(msg.NextHop)
				msg.NextHopAnon = 1
			}
		}
		segment.Out <- msg
	}
}

func init() {
	segment := &Anonymize{}
	segments.RegisterSegment("anonymize", segment)
}
