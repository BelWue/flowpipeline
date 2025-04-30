// Consumes flows from a Kafka instance and passes them to the following
// segments. This segment is based on the kafkaconnector library:
// https://github.com/bwNetFlow/kafkaconnector
package kafkaconsumer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
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
	Server       string        // required
	Topic        string        // required
	Group        string        // required
	User         string        // required if auth is true
	Pass         string        // required if auth is true
	Tls          bool          // optional, default is true
	Auth         bool          // optional, default is true
	StartAt      string        // optional, one of "oldest" or "newest", default is "newest"
	Timeout      time.Duration // optional, default is 15s, any parsable duration
	Legacy       bool          //optional, default is false
	KafkaVersion string        //optional, default is 3.8.0

	startingOffset int64
	saramaConfig   *sarama.Config
	shutdown       chan bool //required for gracefull shutdown
}

func (segment KafkaConsumer) New(config map[string]string) segments.Segment {
	var err error
	newsegment := &KafkaConsumer{}
	newsegment.saramaConfig = sarama.NewConfig()
	newsegment.saramaConfig.ClientID, err = os.Hostname()
	if err != nil {
		log.Warn().Err(err).Msg("KafkaConsumer: failed to fetch hostname")
		newsegment.saramaConfig.ClientID = "unknown"
	}
	newsegment.shutdown = make(chan bool)

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

	if config["strategy"] != "" {
		strategies := []sarama.BalanceStrategy{}
		for _, strategy := range strings.Split(config["strategy"], ",") {
			switch strategy {
			case sarama.StickyBalanceStrategyName:
				strategies = append(strategies, sarama.NewBalanceStrategySticky())
			case sarama.RoundRobinBalanceStrategyName:
				strategies = append(strategies, sarama.NewBalanceStrategyRoundRobin())
			case sarama.RangeBalanceStrategyName:
				strategies = append(strategies, sarama.NewBalanceStrategyRange())
			default:
				log.Warn().Msgf("KafkaConsumer: Unrecognized balance strategy: %s", strategy)
			}
		}
		newsegment.saramaConfig.Consumer.Group.Rebalance.GroupStrategies = strategies
	} else {
		log.Info().Msg("KafkaConsumer:  No Balancing Strategy set - using default \"sticky\"")
		newsegment.saramaConfig.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategySticky()}
	}
	newsegment.saramaConfig.Consumer.Return.Errors = true

	if config["kafka-version"] != "" {
		newsegment.saramaConfig.Version, err = sarama.ParseKafkaVersion(config["kafka-version"])
		if err != nil {
			log.Warn().Err(err).Msgf("KafkaConsumer: Error parsing Kafka version %s - using default %s", newsegment.KafkaVersion, sarama.V3_8_0_0.String())
		} else {
			newsegment.KafkaVersion = config["kafka-version"]
		}
	} else {
		log.Info().Msgf("KafkaConsumer: Using default kafka-version %s", sarama.V3_8_0_0.String())
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
			log.Panic().Err(err).Msg("KafkaConsumer: TLS Error")
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

func (segment *KafkaConsumer) Close() {
	log.Info().Msg("KafkaConsumer: Received connection shutdown command")
	segment.shutdown <- true
	close(segment.shutdown)
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
	} else {
		log.Trace().Msg("KafkaConsumer: Sucessfully Created Kafka client")
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
		// TODO: make the retry intervall/amount configurable
		maxRetries := 12
		retries := 0
		retryInterval := time.Duration(5 * time.Second)
		log.Info().Msg("KafkaConsumer: Establishing connection")
	connectionRetryLoop:
		for {
			// This loop ensures recreation of our consumer session when server-side rebalances happen.
			// check if context was cancelled, signaling that the consumer should stop
			if handlerCtx.Err() != nil {
				return
			}
			if err := client.Consume(handlerCtx, strings.Split(segment.Topic, ","), handler); err != nil {
				log.Info().Err(err).Msgf("KafkaConsumer: Failed to consume kafka topics %s", segment.Topic)
				if retries < maxRetries {
					retries += 1
					log.Error().Err(err).Msgf("KafkaConsumer: Could not create new consumer session (retry %d/%d in %s) due to",
						retries, maxRetries, retryInterval)
					select {
					case <-time.After(retryInterval):
						continue connectionRetryLoop
					case <-segment.shutdown:
						log.Info().Msg("KafkaConsumer: Aborting connection attempt")
						handler.ready <- false
						break connectionRetryLoop
					case <-handlerCtx.Done():
						return
					}
				} else {
					log.Fatal().Err(err).Msg("KafkaConsumer: Could not create new consumer session due to:")
					handler.ready <- false
					return
				}
			}
		}
	}()
	defer func() {
		handlerWg.Wait()
		if err = client.Close(); err != nil {
			log.Error().Err(err).Msg("KafkaConsumer: Error closing Kafka client:")
		}
	}()
	go func() {
		kafkaErr := <-client.Errors()
		if kafkaErr != nil {
			log.Warn().Err(kafkaErr).Msg("KafaConsumer: kafka error")
		}
	}()
	handlerReady := <-handler.ready
	if !handlerReady {
		log.Error().Msg("KafkaConsumer: Failed to establish connection.")
		handlerCancel()
		return
	}
	log.Info().Msg("KafkaConsumer: Connected and operational.")

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
		case <-segment.shutdown:
			log.Info().Msg("KafkaConsumer: Closing connection")
			handlerCancel()
			log.Info().Msg("KafkaConsumer: Connection Closed")
			return
		case handlerReady = <-handler.ready:
			if !handlerReady {
				log.Error().Msg("KafkaConsumer: Failed to establish connection.")
				handlerCancel()
				return
			}
			log.Info().Msg("KafkaConsumer: Reconnected and operational.")
		}
	}
}

func init() {
	segment := &KafkaConsumer{}
	segments.RegisterSegment("kafkaconsumer", segment)
}
