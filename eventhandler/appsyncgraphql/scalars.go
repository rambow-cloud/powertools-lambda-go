package appsyncgraphql

import (
	"crypto/rand"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// MakeID returns a cryptographically random RFC 4122 version 4 identifier.
func MakeID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:]), nil
}

// AWSTimestamp floors seconds since the Unix epoch, including negative instants.
// Scalar helpers take an explicit clock value to keep applications deterministic.
func AWSTimestamp(now time.Time) int64 { return now.Unix() }

func AWSDate(now time.Time, timezoneOffset ...float64) (string, error) {
	return formattedTime(now, "%Y-%m-%d", timezoneOffset)
}
func AWSTime(now time.Time, timezoneOffset ...float64) (string, error) {
	return formattedTime(now, "%H:%M:%S.%f", timezoneOffset)
}
func AWSDateTime(now time.Time, timezoneOffset ...float64) (string, error) {
	return formattedTime(now, "%Y-%m-%dT%H:%M:%S.%f", timezoneOffset)
}

func formattedTime(now time.Time, format string, offsets []float64) (string, error) {
	offset := 0.0
	if len(offsets) > 0 {
		offset = offsets[0]
	}
	if offset < -12 || offset > 14 {
		return "", &NamedError{Name: "RangeError", Message: "timezoneOffset must be between -12 and +14 (inclusive)"}
	}
	postfix := "Z"
	if offset != 0 {
		sign := "+"
		if !(offset > 0) {
			sign = "-"
		}
		absolute := math.Abs(offset)
		postfix = sign + scalarNumber(math.Floor(absolute), 2) + ":" + scalarNumber(math.Floor((absolute-math.Floor(absolute))*60), 2) + ":00"
	}
	// JavaScript Date clips fractional milliseconds and has a finite range.
	millis := float64(now.UnixMilli())
	adjusted := millis + offset*3600000
	parts := []string{"%Y", "NaN", "%m", "NaN", "%d", "NaN", "%H", "NaN", "%M", "NaN", "%S", "NaN", ".%f", ".NaN"}
	if math.Abs(millis) <= 8.64e15 && !math.IsNaN(adjusted) && math.Abs(adjusted) <= 8.64e15 {
		date := time.UnixMilli(int64(math.Trunc(adjusted))).UTC()
		parts = []string{"%Y", strconv.Itoa(date.Year()), "%m", fmt.Sprintf("%02d", date.Month()), "%d", fmt.Sprintf("%02d", date.Day()), "%H", fmt.Sprintf("%02d", date.Hour()), "%M", fmt.Sprintf("%02d", date.Minute()), "%S", fmt.Sprintf("%02d", date.Second()), ".%f", fmt.Sprintf(".%03d", date.Nanosecond()/1e6)}
	}
	return strings.NewReplacer(parts...).Replace(format) + postfix, nil
}

func scalarNumber(number float64, width int) string {
	if math.IsNaN(number) {
		return "NaN"
	}
	return fmt.Sprintf("%0*d", width, int(number))
}
