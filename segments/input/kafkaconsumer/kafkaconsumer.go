// Consumes flows from a Kafka instance and passes them to the following
// segments. This segment is based on the kafkaconnector library:
// https://github.com/bwNetFlow/kafkaconnector
package kafkaconsumer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"github.com/IBM/sarama"
)

// FIXME: clean up those todos
type KafkaConsumer struct {
	segments.BaseSegment
	Server  string        // required
	Topic   string        // required
	Group   string        // required
	User    string        // required if auth is true
	Pass    string        // required if auth is true
	Tls     bool          // optional, default is true
	Auth    bool          // optional, default is true
	StartAt string        // optional, one of "oldest" or "newest", default is "newest"
	Timeout time.Duration // optional, default is 15s, any parsable duration
	Legacy  bool          //optional, default is false

	startingOffset int64
	saramaConfig   *sarama.Config
}

func (segment KafkaConsumer) New(config map[string]string) segments.Segment {
	var err error
	newsegment := &KafkaConsumer{}
	newsegment.saramaConfig = sarama.NewConfig()

	if config["server"] == "" || config["topic"] == "" || config["group"] == "" {
		log.Error().Msg("KafkaConsumer: Missing required configuration parameters.")
		return nil
	} else {
		newsegment.Server = config["server"]
		newsegment.Topic = config["topic"]
		newsegment.Group = config["group"]
	}

	var legacy bool = false
	if config["legacy"] != "" {
		if parsedTls, err := strconv.ParseBool(config["legacy"]); err == nil {
			legacy = parsedTls
		} else {
			log.Error().Msg("KafkaConsumer: Could not parse 'legacy' parameter, using default false.")
		}
	} else {
		log.Debug().Msg("KafkaConsumer: 'legacy' set to default false.")
	}
	newsegment.Legacy = legacy

	// set some unconfigurable defaults
	// newsegment.saramaConfig.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategySticky
	newsegment.saramaConfig.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.BalanceStrategySticky}

	// TODO: parse and set kafka version
	newsegment.saramaConfig.Version, err = sarama.ParseKafkaVersion("2.4.0")
	if err != nil {
		log.Panic().Err(err).Msg("Error parsing Kafka version: ")
	}

	// parse config and setup TLS
	var useTls bool = true
	if config["tls"] != "" {
		if parsedTls, err := strconv.ParseBool(config["tls"]); err == nil {
			useTls = parsedTls
		} else {
			log.Error().Msg("KafkaConsumer: Could not parse 'tls' parameter, using default true.")
		}
	} else {
		log.Info().Msg("KafkaConsumer: 'tls' set to default true.")
	}
	newsegment.Tls = useTls
	if newsegment.Tls {
		rootCAs, err := x509.SystemCertPool()
		if err != nil {
			log.Panic().Err(err).Msg("TLS Error: ")
		}
		newsegment.saramaConfig.Net.TLS.Enable = true
		newsegment.saramaConfig.Net.TLS.Config = &tls.Config{RootCAs: rootCAs}
	} else {
		log.Info().Msg("KafkaConsumer: Disabled TLS, operating unencrypted.")
	}

	// parse config and setup auth
	var useAuth bool = true
	if config["auth"] != "" {
		if parsedAuth, err := strconv.ParseBool(config["auth"]); err == nil {
			useAuth = parsedAuth
		} else {
			log.Error().Msg("KafkaConsumer: Could not parse 'auth' parameter, using default true.")
		}
	} else {
		log.Info().Msg("KafkaConsumer: 'auth' set to default true.")
	}

	// parse and configure credentials, if applicable
	if useAuth && (config["user"] == "" || config["pass"] == "") {
		log.Error().Msg("KafkaConsumer: Missing required configuration parameters for auth.")
		return nil
	} else {
		newsegment.User = config["user"]
		newsegment.Pass = config["pass"]
	}

	// use these credentials
	newsegment.Auth = useAuth
	if newsegment.Auth {
		newsegment.saramaConfig.Net.SASL.Enable = true
		newsegment.saramaConfig.Net.SASL.User = newsegment.User
		newsegment.saramaConfig.Net.SASL.Password = newsegment.Pass
		log.Info().Msgf("KafkaConsumer: Authenticating as user '%s'.", newsegment.User)
	} else {
		newsegment.saramaConfig.Net.SASL.Enable = false
		log.Info().Msg("KafkaConsumer: Disabled auth.")
	}

	// warn if we're leaking credentials
	if newsegment.Auth && !newsegment.Tls {
		log.Warn().Msg("KafkaConsumer: Authentication will be done in plain text!")
	}

	// parse and set starting point of fresh consumer groups
	startAt := "newest"
	var startingOffset int64 = sarama.OffsetNewest // see sarama const OffsetNewest
	if config["startat"] != "" {
		if strings.ToLower(config["startat"]) == "oldest" {
			startAt = "oldest"
			startingOffset = sarama.OffsetOldest // see sarama const OffsetOldest
			log.Info().Msg("KafkaConsumer: Starting at oldest flows.")
		} else if strings.ToLower(config["startat"]) != "newest" {
			log.Error().Msg("KafkaConsumer: Could not parse 'startat' parameter, using default 'newest'.")
		}
	} else {
		log.Info().Msg("KafkaConsumer: 'startat' set to default 'newest'.")
	}
	newsegment.startingOffset = startingOffset
	newsegment.saramaConfig.Consumer.Offsets.Initial = startingOffset
	newsegment.StartAt = startAt

	newsegment.Timeout = 15 * time.Second
	if timeout, err := time.ParseDuration(config["timeout"]); err == nil {
		newsegment.Timeout = timeout
		log.Info().Msgf("KafkaConsumer: Set timeout to '%s'.", config["timeout"])
	} else {
		if config["timeout"] != "" {
			log.Warn().Msgf("KafkaConsumer: Bad configuration of timeout, set to default '15s'.")
		} else {
			log.Info().Msgf("KafkaConsumer: Timeout set to default '15s'.")
		}
	}
	newsegment.saramaConfig.Net.DialTimeout = newsegment.Timeout
	return newsegment
}

func (segment *KafkaConsumer) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	client, err := sarama.NewConsumerGroup(strings.Split(segment.Server, ","), segment.Group, segment.saramaConfig)
	if err != nil {
		if client == nil {
			log.Fatal().Err(err).Msg("KafkaConsumer: Creating Kafka client failed, this indicates an unreachable server, invalid credentials, or a SSL problem. Original error:\n  ")
		} else {
			log.Fatal().Err(err).Msg("KafkaConsumer: Creating Kafka consumer group failed while the connection was okay. Original error:\n  ")
		}
	}

	handlerCtx, handlerCancel := context.WithCancel(context.Background())
	var handler = &Handler{
		ready:  make(chan bool),
		flows:  make(chan *pb.EnrichedFlow),
		legacy: segment.Legacy,
	}
	handlerWg := sync.WaitGroup{}
	handlerWg.Add(1)
	go func() {
		defer handlerWg.Done()
		for {
			// This loop ensures recreation of our consumer session when server-side rebalances happen.
			if err := client.Consume(handlerCtx, strings.Split(segment.Topic, ","), handler); err != nil {
				log.Error().Err(err).Msg(" KafkaConsumer: Could not create new consumer session, retry in 5s. Original error:\n  ")
				time.Sleep(5 * time.Second) // TODO: although this never occured for me, make configurable
				continue
			}
			// check if context was cancelled, signaling that the consumer should stop
			if handlerCtx.Err() != nil {
				return
			}
			handler.ready = make(chan bool) // TODO: this is from the official example, not sure it is necessary in out case
		}
	}()
	<-handler.ready
	log.Info().Msg("KafkaConsumer: Connected and operational.")

	defer func() {
		handlerWg.Wait()
		if err = client.Close(); err != nil {
			log.Panic().Err(err).Msg(" KafkaConsumer: Error closing Kafka client: ")
		}
	}()

	// receive flows in a loop
	for {
		select {
		case msg, ok := <-handler.flows:
			if !ok {
				// This will occur during a rebalance when the handler calls its Cleanup method
				continue
			}
			segment.Out <- msg
		case msg, ok := <-segment.In:
			if !ok {
				handlerCancel() // Trigger handler shutdown and cleanup
				return
			} else {
				segment.Out <- msg
			}
		}
	}
}

func init() {
	segment := &KafkaConsumer{}
	segments.RegisterSegment("kafkaconsumer", segment)
}
