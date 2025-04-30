// Anonymize uses the CryptoPan prefix-preserving IP address sanitization as specified by J. Fan, J. Xu, M. Ammar, and S. Moon.
// By default this segment is anonymizing the SrcAddr, DstAddr and SamplerAddress fields in the flowmessage.
// The required encryption key has to be created with the length of 32 chars.
package anonymize

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	cryptopan "github.com/Yawning/cryptopan"
)

type Mode string

const (
	ModeCryptoPan Mode = "cryptopan"
	ModeSubNet    Mode = "subnet"
	ModeAll       Mode = "all"
)

type SubnetAnonymizer struct {
	MaskV4 uint32
	MaskV6 uint32
}

type Anonymize struct {
	segments.BaseSegment
	EncryptionKey     string   // required if AnonymizationMode == cryptopan or AnonymizationMode == All, key for anonymization by Crypto-PAn.
	Fields            []string // optional, list of Fields to anonymize their IP address. Default if not set are all available fields: SrcAddr, DstAddr, SamplerAddress
	AnonymizationMode Mode     //optional, define which mode should be used for anonymizing ips. Default is Crypto-PAn

	cryptopanAnonymizer *cryptopan.Cryptopan
	subnetAnonymizer    *SubnetAnonymizer //requires config fields if AnonymizationMode == subnet or AnonymizationMode == All
}

func (segments Anonymize) New(config map[string]string) segments.Segment {
	var (
		encryptionKey       string
		cryptoPanAnonymizer *cryptopan.Cryptopan
		subnetAnonymizer    *SubnetAnonymizer
		err                 error
	)

	mode, err := modeFromConfig(config["mode"])
	if err != nil {
		log.Error().Msgf("Anonymize: unknown anonymization mode: %s", config["mode"])
		return nil
	}

	if mode == ModeSubNet || mode == ModeAll {
		maskV4 := 16
		maskV6 := 52

		if config["maskV4"] == "" {
			log.Info().Msg("Anonymize: No maskV4 provided for subnet anonymization - using default 16")
		} else {
			maskV4, err = strconv.Atoi(config["maskV4"])
			if err != nil || maskV4 > 32 || maskV4 < 8 {
				log.Error().Msgf("Anonymize: Bad value \"%s\" for argument maskV4 - expected int <=32 && >= 8", config["maskV4"])
				return nil
			}
		}

		if config["maskV6"] == "" {
			log.Info().Msg("Anonymize: No maskV6 provided for subnet anonymization - using default 52")
		} else {
			maskV6, err = strconv.Atoi(config["maskV6"])
			if err != nil || maskV6 > 128 || maskV4 < 4 {
				log.Error().Msgf("Anonymize: Bad value \"%s\" for argument maskV6 - expected int <=128 && >= 4", config["maskV6"])
				return nil
			}
		}

		subnetAnonymizer = &SubnetAnonymizer{
			MaskV4: uint32(maskV4),
			MaskV6: uint32(maskV6),
		}
	}
	if mode == ModeCryptoPan || mode == ModeAll {

		if config["key"] == "" {
			log.Error().Msg("Anonymize: Missing configuration parameter 'key'. Please set the key to use for anonymization of IP addresses.")
			return nil
		} else {
			encryptionKey = config["key"]
		}

		ekb := []byte(encryptionKey)
		cryptoPanAnonymizer, err = cryptopan.New(ekb)
		if err != nil {
			if _, ok := err.(cryptopan.KeySizeError); ok {
				log.Error().Msgf("Anonymize: Key has insufficient length %d, please specify one with more than 32 chars.", len(encryptionKey))
			} else {
				log.Error().Err(err).Msgf("Anonymize: error creating anonymizer")
			}
			return nil
		}
	}

	// set default fields with IPs to anonymize
	var fields = []string{
		"DstAddr",
		"NextHop",
		"SamplerAddress",
		"SrcAddr",
	}
	if config["fields"] == "" {
		log.Info().Msgf("Anonymize: Missing configuration parameter 'fields'. Using default fields '%s' to anonymize.", fields)
	} else {
		fields = []string{}
		for _, field := range strings.Split(config["fields"], ",") {
			log.Info().Msgf("Anonymize: custom field found: \"%s\"", field)
			fields = append(fields, field)
		}
	}

	return &Anonymize{
		EncryptionKey:       encryptionKey,
		cryptopanAnonymizer: cryptoPanAnonymizer,
		subnetAnonymizer:    subnetAnonymizer,
		Fields:              fields,
		AnonymizationMode:   mode,
	}
}

func modeFromConfig(s string) (Mode, error) {
	switch s {
	case "":
		//default == cryptopan for backwards compatibility
		log.Info().Msgf("Anonymize: no anonymization mode specified: using default \"cryptopan\"")
		return ModeCryptoPan, nil
	case "cryptopan":
		return ModeCryptoPan, nil
	case "subnet":
		return ModeSubNet, nil
	case "all":
		return ModeAll, nil
	default:
		return "", fmt.Errorf("invalid value: %s", s)
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
				msg.SrcAddr, msg.SrcAddrAnon, msg.SrcAddrPreservedLen = segment.anonymize(msg.SrcAddr, msg.SrcAddrPreservedLen)
			case "DstAddr":
				if msg.DstAddrObj() == nil {
					continue
				}
				msg.DstAddr, msg.DstAddrAnon, msg.DstAddrPreservedLen = segment.anonymize(msg.DstAddr, msg.DstAddrPreservedLen)
			case "SamplerAddress":
				if msg.SamplerAddressObj() == nil {
					continue
				}
				msg.SamplerAddress, msg.SamplerAddrAnon, msg.SamplerAddrPreservedPrefixLen = segment.anonymize(msg.SamplerAddress, 0)
			case "NextHop":
				if msg.NextHopObj() == nil {
					continue
				}
				msg.NextHop, msg.NextHopAnon, msg.NextHopAnonPreservedPrefixLen = segment.anonymize(msg.NextHop, msg.NextHopAnonPreservedPrefixLen)
			}
		}
		segment.Out <- msg
	}
}

func (s *Anonymize) anonymize(ip net.IP, addrPreservedLen uint32) (net.IP, pb.EnrichedFlow_AnonymizedType, uint32) {
	switch s.AnonymizationMode {
	case ModeCryptoPan:
		return s.cryptopanAnonymizer.Anonymize(ip), pb.EnrichedFlow_CryptoPAN, addrPreservedLen
	case ModeSubNet:
		ip, addrPreservedLen = s.subnetAnonymizer.reduceIPToSubnet(ip, addrPreservedLen)
		return ip, pb.EnrichedFlow_Subnet, addrPreservedLen
	case ModeAll:
		ip = s.cryptopanAnonymizer.Anonymize(ip)
		ip, addrPreservedLen = s.subnetAnonymizer.reduceIPToSubnet(ip, addrPreservedLen)
		return ip, pb.EnrichedFlow_SubnetAndCryptoPAN, addrPreservedLen
	}

	return ip, pb.EnrichedFlow_NotAnonymized, addrPreservedLen
}

func (anon SubnetAnonymizer) reduceIPToSubnet(ip net.IP, originalAddrPreservedLen uint32) (net.IP, uint32) {

	var (
		preservedLen uint32
		mask         net.IPMask
	)
	if ip.To4() != nil {
		mask = net.CIDRMask(int(anon.MaskV4), 32)
		if originalAddrPreservedLen == 0 || anon.MaskV4 < originalAddrPreservedLen {
			preservedLen = anon.MaskV4
		} else {
			preservedLen = originalAddrPreservedLen
		}
	} else {
		mask = net.CIDRMask(int(anon.MaskV4), 128)
		if originalAddrPreservedLen == 0 || anon.MaskV6 < originalAddrPreservedLen {
			preservedLen = anon.MaskV6
		} else {
			preservedLen = originalAddrPreservedLen
		}
	}
	return ip.Mask(mask), preservedLen
}

func init() {
	segment := &Anonymize{}
	segments.RegisterSegment("anonymize", segment)
}
