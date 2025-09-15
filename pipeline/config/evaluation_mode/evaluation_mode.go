package evaluation_mode

import "strings"

type EvaluationMode uint8

const (
	Unknown EvaluationMode = iota
	Destination
	SourceAndDestination
	Source
	Connection
)

func (s EvaluationMode) String() string {
	return [...]string{"", "destination", "source and destination", "source", "connection"}[s]
}

var (
	evaluationModes = map[string]EvaluationMode{
		"":                       Unknown,
		"destination":            Destination,
		"destination and source": SourceAndDestination, //bad input compability
		"source and destination": SourceAndDestination,
		"source":                 Source,
		"connection":             Connection,
	}
)

func ParseEvaluationMode(str string) EvaluationMode {
	r, ok := evaluationModes[strings.ToLower(str)]
	if ok {
		return r
	}
	return Unknown
}

func (a EvaluationMode) MarshalText() ([]byte, error) {
	return []byte(a.String()), nil
}

func (a *EvaluationMode) UnmarshalText(text []byte) error {
	*a = ParseEvaluationMode(string(text))
	return nil
}
