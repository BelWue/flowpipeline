package influx

import (
	"context"
	"fmt"
	"log"
	"net"
	"reflect"
	"strconv"
	"time"

	"github.com/bwNetFlow/flowpipeline/pb"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

// Connector provides export features to Influx
type Connector struct {
	Address      string
	Org          string
	Bucket       string
	Token        string
	ExportFreq   int
	Batchsize    int
	Tags         []string
	Fields       []string
	influxClient influxdb2.Client
}

// Initialize a connection to Influxdb
func (c *Connector) Initialize() {
	c.influxClient = influxdb2.NewClientWithOptions(
		c.Address,
		c.Token,
		influxdb2.DefaultOptions().SetBatchSize(uint(c.Batchsize)))

	c.checkBucket()
}

// check if database exists
func (c *Connector) checkBucket() {
	bucket, err := c.influxClient.BucketsAPI().FindBucketByName(context.Background(), c.Bucket)
	if err != nil {
		// The bucket should be created by the Influxdb admin.
		log.Printf("[warning] Influx: Given bucket %s not found.", c.Bucket)
	} else {
		log.Printf("[info] Influx: Bucket found with result: %s", bucket.Name)
	}
}

func (c *Connector) CreatePoint(msg *pb.EnrichedFlow) *write.Point {
	// write tags for datapoint and drop them to not insert as fields
	tags := make(map[string]string)
	values := reflect.ValueOf(msg).Elem()
	for _, tagname := range c.Tags {
		value := values.FieldByName(tagname).Interface()
		switch value.(type) {
		case []uint8: // this is necessary for proper formatting
			ipstring := net.IP(value.([]uint8)).String()
			if ipstring == "<nil>" {
				ipstring = ""
			}
			tags[tagname] = ipstring
		case uint32: // this is because FormatUint is much faster than Sprint
			tags[tagname] = strconv.FormatUint(uint64(value.(uint32)), 10)
		case uint64: // this is because FormatUint is much faster than Sprint
			tags[tagname] = strconv.FormatUint(uint64(value.(uint64)), 10)
		case string: // this is because doing nothing is also much faster than Sprint
			tags[tagname] = value.(string)
		default:
			tags[tagname] = fmt.Sprint(value)
		}
	}

	fields := make(map[string]interface{})
	for _, fieldname := range c.Fields {
		fields[fieldname] = values.FieldByName(fieldname).Interface()
	}

	// create point
	p := influxdb2.NewPoint(
		"flowdata",
		tags,
		fields,
		time.Now())
	return p
}
