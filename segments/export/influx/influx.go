// Collects and exports all flows to influxdb for long term storage.
// Tags to configure for Influxdb are from the protobuf definition.
// Supported Tags are:
// Cid,ProtoName,RemoteCountry,SamplerAddress,SrcIfDesc,DstIfDesc
// If no Tags are provided 'ProtoName' will be the only Tag used by default.
package influx

import (
	"net/url"
	"reflect"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
)

type Influx struct {
	segments.BaseSegment
	Address string   // optional, URL for influxdb endpoint, default is http://127.0.0.1:8086
	Org     string   // required, Influx org name
	Bucket  string   // required, Influx bucket
	Token   string   // required, Influx access token
	Tags    []string // optional, list of Tags to be created.
	Fields  []string // optional, list of Fields to be created, default is "Bytes,Packets"
}

func (segment Influx) New(config map[string]string) segments.Segment {
	newsegment := &Influx{}

	// TODO: add paramteres for Influx endpoint and eval vars
	if config["address"] != "" {
		// check if a valid url has been passed
		_, err := url.Parse(config["address"])
		if err != nil {
			log.Error().Err(err).Msg("Influx: error parsing given url")
		}
		newsegment.Address = config["address"]
	} else {
		newsegment.Address = "http://127.0.0.1:8086"
		log.Info().Msg("Influx: Missing configuration parameter 'address'. Using default address 'http://127.0.0.1:8086'")
	}

	if config["org"] == "" {
		log.Error().Msg("Influx: Missing configuration parameter 'org'. Please set the organization to use.")
		return nil
	} else {
		newsegment.Org = config["org"]
	}

	if config["bucket"] == "" {
		log.Error().Msg("Influx: Missing configuration parameter 'bucket'. Please set the bucket to use.")
		return nil
	} else {
		newsegment.Bucket = config["bucket"]
	}

	if config["token"] == "" {
		log.Error().Msg("Influx: Missing configuration parameter 'token'. Please set the token to use.")
		return nil
	} else {
		newsegment.Token = config["token"]
	}

	// set default Tags if not configured
	if config["tags"] == "" {
		log.Info().Msg("Influx: Configuration parameter 'tags' not set. Using default tags 'ProtoName' to export.")
		newsegment.Tags = []string{"ProtoName"}
	} else {
		newsegment.Tags = strings.Split(config["tags"], ",")
		protomembers := reflect.TypeOf(pb.EnrichedFlow{})
		for _, tagname := range newsegment.Tags {
			tagname = strings.TrimSpace(tagname)
			_, found := protomembers.FieldByName(tagname)
			if !found {
				log.Error().Msgf("Influx: Unknown name '%s' specified in 'tags'.", tagname)
				return nil
			}
		}
	}

	// set default Fields if not configured
	if config["fields"] == "" {
		log.Info().Msg("Influx: Configuration parameter 'fields' not set. Using default fields 'Bytes,Packets' to export.")
		newsegment.Fields = []string{"Bytes", "Packets"}
	} else {
		newsegment.Fields = strings.Split(config["fields"], ",")
		for _, fieldname := range newsegment.Fields {
			protomembers := reflect.TypeOf(pb.EnrichedFlow{})
			_, found := protomembers.FieldByName(fieldname)
			if !found {
				log.Error().Msgf("Influx: Unknown name '%s' specified in 'fields'.", fieldname)
				return nil
			}
		}
	}

	return newsegment
}

func (segment *Influx) Run(wg *sync.WaitGroup) {
	// TODO: extend options
	var connector = Connector{
		Address:   segment.Address,
		Bucket:    segment.Bucket,
		Org:       segment.Org,
		Token:     segment.Token,
		Batchsize: 5000,
		Tags:      segment.Tags,
		Fields:    segment.Fields,
	}

	// initialize Influx endpoint
	connector.Initialize()
	writeAPI := connector.influxClient.WriteAPI(connector.Org, connector.Bucket)
	defer func() {
		close(segment.Out)
		// Force all unwritten data to be sent
		writeAPI.Flush()
		connector.influxClient.Close()
		wg.Done()
	}()

	for msg := range segment.In {
		segment.Out <- msg
		datapoint := connector.CreatePoint(msg)
		if datapoint == nil {
			// just ignore raised warnings if flow cannot be converted or unmarshalled
			continue
		}
		// async write
		writeAPI.WritePoint(datapoint)
	}
}

func init() {
	segment := &Influx{}
	segments.RegisterSegment("influx", segment)
}
