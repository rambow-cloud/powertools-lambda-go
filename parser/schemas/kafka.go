package schemas

import (
	"context"
	"fmt"
	"math"
	"strings"
	"unicode/utf16"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

var kafkaText = parser.Transform(parser.String(), func(_ context.Context, input any) (any, error) {
	return commons.DecodeUTF8(commons.DecodeBase64Buffer(input.(string))), nil
})
var kafkaHeader = parser.Transform(array(parser.Number()), func(_ context.Context, input any) (any, error) {
	values := input.([]any)
	units := make([]uint16, 0, len(values))
	for _, value := range values {
		n := value.(float64)
		if n < 0 || n > 0x10ffff || math.Trunc(n) != n {
			return nil, fmt.Errorf("invalid Kafka header code point: %v", n)
		}
		if n <= 0xffff {
			units = append(units, uint16(n))
		} else {
			high, low := utf16.EncodeRune(rune(n))
			units = append(units, uint16(high), uint16(low))
		}
	}
	return string(utf16.Decode(units)), nil
})
var KafkaRecordSchema = parser.Object(field("topic", parser.String()), field("partition", parser.Number()), field("offset", parser.Number()), field("timestamp", parser.Number()), field("timestampType", parser.String()), optional("key", kafkaText), field("value", kafkaText), field("headers", array(parser.Dictionary(kafkaHeader))))
var kafkaBase = parser.Object(nullish("bootstrapServers", parser.Transform(parser.String(), func(_ context.Context, input any) (any, error) { return strings.Split(input.(string), ","), nil })), field("records", parser.Dictionary(array(KafkaRecordSchema, 1))))
var KafkaSelfManagedEventSchema = kafkaBase.Extend(field("eventSource", parser.Literal("SelfManagedKafka")))
var KafkaMskEventSchema = kafkaBase.Extend(field("eventSource", parser.Literal("aws:kafka")), field("eventSourceArn", parser.String()))
