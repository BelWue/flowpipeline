package utils

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"strings"

	_ "embed"
)

const largestIanaProtoNameIndex = 256

var (
	IanaProtocolNumberNames          [largestIanaProtoNameIndex]string
	IanaProtocolNumberLowercaseNames [largestIanaProtoNameIndex]string
)

// source: https://www.iana.org/assignments/protocol-numbers/protocol-numbers.xhtml
//
//go:embed res/iana/protocol-numbers-1.csv
var ianaProtocolNumbersCSV []byte

func init() {
	// read IANA Protocol Numbers
	csvRows, err := csv.NewReader(bytes.NewBuffer(ianaProtocolNumbersCSV)).ReadAll()
	if err != nil {
		panic(err)
	}
	for _, row := range csvRows {
		// skip mangled rows
		if len(row) < 3 {
			continue
		}
		idx, err := strconv.Atoi(row[0])
		// skip unparsable proto numbers
		if err != nil {
			continue
		}

		if row[1] != "" {
			IanaProtocolNumberNames[idx] = row[1]
			IanaProtocolNumberLowercaseNames[idx] = strings.ToLower(row[1])
			// use description if name is empty
		} else {
			IanaProtocolNumberNames[idx] = row[2]
			IanaProtocolNumberLowercaseNames[idx] = strings.ToLower(row[2])
		}
	}
}

// IanaProtocolNumberToName returns the IANA assigned name to a Layer 4 protocol number
func IanaProtocolNumberToName(protocolNumber uint32) string {
	if protocolNumber > largestIanaProtoNameIndex {
		return ""
	}
	return IanaProtocolNumberNames[protocolNumber]
}

// IanaProtocolNumberToLowercaseName returns the IANA assigned name (lowercase) to a Layer 4 protocol number
func IanaProtocolNumberToLowercaseName(protocolNumber uint32) string {
	if protocolNumber > largestIanaProtoNameIndex {
		return ""
	}
	return IanaProtocolNumberLowercaseNames[protocolNumber]
}
