package schemas

import (
	"context"
	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"math"
)

var positiveInteger = parser.Pipe(parser.Number(), parser.SchemaFunc[any](func(ctx context.Context, input any) (any, []parser.Issue, error) {
	value := input.(float64)
	if math.Trunc(value) != value {
		return nil, []parser.Issue{{Code: "invalid_type", Expected: "int", Message: "Invalid input: expected int, received number"}}, nil
	}
	var issues []parser.Issue
	if value > 9007199254740991 {
		issues = append(issues, parser.Issue{Code: "too_big", Message: "Too big: expected int to be <9007199254740991", Continuable: true})
	}
	if value < -9007199254740991 {
		issues = append(issues, parser.Issue{Code: "too_small", Message: "Too small: expected int to be >-9007199254740991", Continuable: true})
	}
	output, positiveIssues, err := positiveNumber.Validate(ctx, input)
	issues = append(issues, positiveIssues...)
	return output, issues, err
}))
var sesVerdict = parser.Object(field("status", parser.Enum("PASS", "FAIL", "GRAY", "PROCESSING_FAILED")))
var sesReceipt = parser.Object(field("timestamp", isoDateTime), field("processingTimeMillis", positiveInteger), field("recipients", array(parser.String())), field("spamVerdict", sesVerdict), field("virusVerdict", sesVerdict), field("spfVerdict", sesVerdict), field("dmarcVerdict", sesVerdict), field("dkimVerdict", sesVerdict), field("dmarcPolicy", parser.Enum("none", "quarantine", "reject")), field("action", parser.Object(field("type", parser.Enum("Lambda")), field("invocationType", parser.Literal("Event")), field("functionArn", parser.String()))))
var sesMail = parser.Object(
	field("timestamp", isoDateTime), field("source", parser.String()), field("messageId", parser.String()), field("destination", array(parser.String())), field("headersTruncated", parser.Boolean()), field("headers", array(parser.Object(field("name", parser.String()), field("value", parser.String())))),
	field("commonHeaders", parser.Object(field("from", array(parser.String())), field("to", array(parser.String())), optional("cc", array(parser.String())), optional("bcc", array(parser.String())), optional("sender", array(parser.String())), optional("reply-to", array(parser.String())), field("returnPath", parser.String()), field("messageId", parser.String()), field("date", parser.String()), field("subject", parser.String()))),
)
var SesRecordSchema = parser.Object(field("eventSource", parser.Literal("aws:ses")), field("eventVersion", parser.String()), field("ses", parser.Object(field("mail", sesMail), field("receipt", sesReceipt))))
var SesSchema = parser.Object(field("Records", array(SesRecordSchema, 1)))
