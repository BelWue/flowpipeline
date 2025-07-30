// Filters out the bulky average of flows.
package elephant

import (
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BelWue/flowpipeline/segments"
	"github.com/rs/zerolog/log"

	"github.com/asecurityteam/rolling"
)

type Elephant struct {
	segments.BaseFilterSegment
	Aspect     string  // optional, one of "bytes", "bps", "packets", or "pps", default is "bytes", determines which aspect qualifies a flow as an elephant
	Percentile float64 // optional, default is 99.00, determines the cutoff percentile for flows being dropped by this segment, i.e. 95.00 corresponds to outputting the top 5% only
	// TODO: add option to get bottom percent?
	Exact      bool // optional, default is false, determines whether to use percentiles that are exact or generated using the P-square estimation algorithm
	Window     int  // optional, default is 300, sets the number of seconds used as a sliding window size
	RampupTime int  // optional, default is 0, sets the time to wait for analyzing flows. All flows within this Timerange are dropped.
}

func (segment Elephant) New(config map[string]string) segments.Segment {
	var aspect = "bytes"
	if config["aspect"] != "" {
		if strings.ToLower(config["aspect"]) == "bytes" || strings.ToLower(config["aspect"]) == "packets" || strings.ToLower(config["aspect"]) == "bps" || strings.ToLower(config["aspect"]) == "pps" {
			aspect = strings.ToLower(config["aspect"])
		} else {
			log.Error().Msg("Elephant: Could not parse 'aspect' parameter, using default 'bytes'.")
		}
	} else {
		log.Info().Msg("Elephant: 'aspect' set to default 'bytes'.")
	}

	var percentile = 99.00
	if config["percentile"] != "" {
		if parsedPercentile, err := strconv.ParseFloat(config["percentile"], 64); err == nil {
			percentile = parsedPercentile
			if percentile == 0 {
				log.Error().Msg("Elephant: Using 0-Percentile corresponds to no-op. Remove this segment or use a higher value.")
				return nil
			}
		} else {
			log.Error().Msg("Elephant: Could not parse 'percentile' parameter, using default 99.00.")
		}
	} else {
		log.Info().Msg("Elephant: 'percentile' set to default 99.00.")
	}

	var exact = false
	if config["exact"] != "" {
		if parsedExact, err := strconv.ParseBool(config["exact"]); err == nil {
			exact = parsedExact
		} else {
			log.Error().Msg("Elephant: Could not parse 'exact' parameter, using default false.")
		}
	} else {
		log.Info().Msg("Elephant: 'exact' set to default false.")
	}

	var window = 300
	if config["window"] != "" {
		if parsedWindow, err := strconv.ParseInt(config["window"], 10, 64); err == nil {
			if parsedWindow <= 0 {
				log.Error().Msg("Elephant: Window has to be >0.")
				return nil
			}
			if parsedWindow > math.MaxInt {
				log.Error().Msg("Elephant: Window out of range.")
				return nil
			}
			window = int(parsedWindow)
		} else {
			log.Error().Msg("Elephant: Could not parse 'window' parameter, using default 300.")
		}
	} else {
		log.Info().Msg("Elephant: 'window' set to default 300.")
	}

	var rampuptime = 0
	if config["rampuptime"] != "" {
		if ramptime, err := strconv.ParseInt(config["rampuptime"], 10, 64); err == nil {
			if ramptime < 0 {
				log.Error().Msg("Elephant: Rampuptime has to be >= 0.")
				return nil
			}
			if ramptime > math.MaxInt {
				log.Error().Msg("Elephant: Rampuptime out of range.")
				return nil
			}
			rampuptime = int(ramptime)
		} else {
			log.Error().Msg("Elephant: Could not parse 'rampuptime' parameter, using default 0.")
		}
	} else {
		log.Info().Msg("Elephant: 'rampuptime' set to default 0.")
	}

	return &Elephant{
		Aspect:     aspect,
		Percentile: percentile,
		Exact:      exact,
		Window:     window,
		RampupTime: rampuptime,
	}
}

func (segment *Elephant) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	var inRampup bool
	var rampupEnd time.Time
	if segment.RampupTime > 0 {
		inRampup = true
		rampupEnd = time.Now().Add(time.Duration(segment.RampupTime) * time.Second)
	}
	var window = rolling.NewTimePolicy(rolling.NewWindow(segment.Window), time.Second)
	for msg := range segment.In {
		// always determine a flow's aspect to append to the window
		var aspect float64
		switch segment.Aspect {
		case "bytes":
			aspect = float64(msg.Bytes)
		case "bps":
			duration := msg.TimeFlowEnd - msg.TimeFlowStart
			if duration == 0 {
				duration += 1
			}
			aspect = float64(msg.Bytes * 8.0 / duration)
		case "pps":
			duration := msg.TimeFlowEnd - msg.TimeFlowStart
			if duration == 0 {
				duration += 1
			}
			aspect = float64(msg.Packets / duration)
		case "packets":
			aspect = float64(msg.Packets)
		}
		window.Append(aspect)

		// Check if ramp up phase is over. Shortcircuiting avoids
		// permanent checks against time.Now().
		if inRampup && time.Now().After(rampupEnd) {
			inRampup = false
			log.Info().Msg("Elephant: RampupTime complete, passing through flows now.")
		}
		// Only do expensive threshold calculation when out of ramp up
		// phase.
		// This checks inRampup a second time instead of just using
		// else with the previous if to ensure the first flow after
		// rampupEnd is considered.
		if !inRampup {
			var threshold float64
			if segment.Exact {
				threshold = window.Reduce(rolling.Percentile(segment.Percentile))
			} else {
				threshold = window.Reduce(rolling.FastPercentile(segment.Percentile))
			}
			if aspect >= threshold {
				log.Debug().Msgf("Elephant: Found elephant with size %d (>=%f)", msg.Bytes, threshold)
				segment.Out <- msg
				continue
			}
		}
		// implicit "(if inRampup || aspect < threshold) && ..." due to the continue 3 lines above
		if segment.Drops != nil {
			segment.Drops <- msg
		}
	}
}

func init() {
	segment := &Elephant{}
	segments.RegisterSegment("elephant", segment)
}
