// The `lumberjack` segment sends flows to one or more [elastic beats](https://github.com/elastic/beats)
// servers user the [lumberjack](https://github.com/logstash-plugins/logstash-input-beats/blob/main/PROTOCOL.md)
// protocol. Flows are queued in a non-deterministic, round-robin fashion to the servers.
//
// The only mandatory option is `servers` which contains a comma separated list of lumberjack
// server URLs. Each URL must start with one of these schemata: `tcp://` (plain TCP,
// no encryption), `tls://` (TLS encryption) or `tlsnoverify://` (TLS encryption without
// certificate verification). The schema is followed by the hostname or IP address, a colon `:`,
// and a port number. IPv6 addresses must be surrounded by square brackets.
//
// A goroutine is spawned for every lumberjack server. Each goroutine only uses one CPU core to
// process and send flows. This may not be enough when the ingress flow rate is high and/or a high compression
// level is used. The number of goroutines per backend can by set explicitly with the `?count=x` URL
// parameter. For example:
//
// ```yaml
// config:
//   server: tls://host1:5043/?count=4, tls://host2:5043/?compression=9&count=16
// ```
//
// will use four parallel goroutines for `host1` and sixteen parallel goroutines for `host2`. Use `&count=…` instead of
// `?count=…` when `count` is not the first parameter (standard URI convention).
//
// Transport compression is disabled by default. Use `compression` to set the compression level
// for all hosts. Compression levels can vary between 0 (no compression) and 9 (maximum compression).
// To set per-host transport compression adding `?compression=<level>` to the server URI.
//
// To prevent blocking, flows are buffered in a channel between the segment and the output
// go routines. Each output go routine maintains a buffer of flows which are send either when the
// buffer is full or after a configurable timeout. Proper parameter sizing for the queue,
// buffers, and timeouts depends on multiple individual factors (like size, characteristics
// of the incoming netflows and the responsiveness of the target servers). There are parameters
// to both observe and tune this segment's performance.
//
// Upon connection error or loss, the segment will try to reconnect indefinitely with a pause of
// `reconnectwait` between attempts.
//
// * `queuesize` (integer) sets the number of flows that are buffered between the segment and the output go routines.
// * `batchsize` (integer) sets the number of flows that each output go routine buffers before sending.
// * `batchtimeout` (duration) sets the maximum time that flows are buffered before sending.
// * `reconnectwait` (duration) sets the time to wait between reconnection attempts.
//
// These options help to observe the performance characteristics of the segment:
//
// * `batchdebug` (bool) enables debug logging of batch operations (full send, partial send and skipped send).
// * `queuestatusinterval` (duration) sets the interval at which the segment logs the current queue status.
//
// To see debug output, set the `-l debug` flag when starting `flowpipeline`.
//
// See [time.ParseDuration](https://pkg.go.dev/time#ParseDuration) for proper duration format
// strings and [strconv.ParseBool](https://pkg.go.dev/strconv#ParseBool) for allowed bool keywords.
package lumberjack

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"time"

	"github.com/BelWue/flowpipeline/pb"
	lumber "github.com/elastic/go-lumber/client/v2"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/encoding/protojson"
)

type resilientClient struct {
	sc            *lumber.SyncClient
	ServerName    string
	Options       ServerOptions
	ReconnectWait time.Duration
}

func NewResilientClient(serverName string, options ServerOptions, reconnectWait time.Duration) *resilientClient {
	return &resilientClient{
		ServerName:    serverName,
		Options:       options,
		ReconnectWait: reconnectWait,
	}
}

// wrapper function to use protobuf json encoder messages when possible
func jsonEncoderWrapper(msg interface{}) ([]byte, error) {
	protoMsg, ok := msg.(*pb.EnrichedFlow)
	if ok {
		return protojson.Marshal(protoMsg)
	} else {
		return json.Marshal(msg)
	}
}

// connect will attempt to connect to the server and will retry indefinitely if the connection fails.
func (c *resilientClient) connect() {
	var err error
	// built function that implements TLS options
	dialFunc := func(network string, address string) (net.Conn, error) {
		if c.Options.UseTLS {
			return tls.Dial(network, address,
				&tls.Config{
					InsecureSkipVerify: !c.Options.VerifyCertificate,
				})
		} else {
			return net.Dial(network, address)
		}
	}
	// try connecting indefinitely
	for {
		c.sc, err = lumber.SyncDialWith(dialFunc, c.ServerName, lumber.JSONEncoder(jsonEncoderWrapper), lumber.CompressionLevel(c.Options.CompressionLevel))
		if err == nil {
			return
		}
		log.Error().Err(err).Msgf("Lumberjack: Failed to connect to server %s", c.ServerName)
		time.Sleep(c.ReconnectWait)
	}
}

// Send will try to send the given events to the server. If the connection fails, it will retry indefinitely.
// If the connection is lost or never exists, it will reconnect until a connection is established.
func (c *resilientClient) Send(events []interface{}) {
	// connect on first send when no client exists
	if c.sc == nil {
		c.connect()
	}
	for {
		// send events, return on success
	sendEvents:
		_, err := c.sc.Send(events)
		if err == nil {
			return
		}

		// connection is closed. Reopen connection and retry
		if err == io.EOF {
			log.Error().Err(err).Msgf("Lumberjack: Connection to server %s closed by peer", c.ServerName)
			_ = c.sc.Close()
			c.connect()
			goto sendEvents
		}

		// retry on timeout
		if netError, ok := err.(net.Error); ok && netError.Timeout() {
			goto sendEvents
		}

		log.Error().Err(err).Msgf("Lumberjack: Error sending flows to server %s", c.ServerName)
		time.Sleep(500 * time.Millisecond) // TODO: implement a better retry strategy

		// unexpected error. Close connection and retry.
		{
			log.Error().Msgf("Lumberjack: Unexpected error while sending to %s. Restarting connection…", c.ServerName)
			_ = c.sc.Close()
			c.connect()
			goto sendEvents
		}

	}
}

// SendNoRetry will try to send the given events to the server. If the connection fails, it will not retry.
func (c *resilientClient) SendNoRetry(events []interface{}) (int, error) {
	return c.sc.Send(events)
}

// Close will close the connection to the server.
func (c *resilientClient) Close() {
	err := c.sc.Close()
	if err != nil {
		log.Error().Err(err).Msgf("Lumberjack: Error closing connection to server %s", c.ServerName)
	}
}
