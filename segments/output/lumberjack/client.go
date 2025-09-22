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

func newResilientClient(serverName string, options ServerOptions, reconnectWait time.Duration) *resilientClient {
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
		return protojson.Marshal(struct {
			*pb.EnrichedFlow
			TimeStamp uint64 `json:"@timestamp"`
		}{
			protoMsg,
			protoMsg.TimeFlowStart,
		})
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
