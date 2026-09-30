package http

import (
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

// ClientIP returns the event's source IP for API Gateway and Function URLs.
// ALB uses the first X-Forwarded-For value. This is request metadata, not a
// trusted authorization input; the source and forwarding headers are caller-controlled.
func (r *RequestContext) ClientIP() string {
	request := object(object(r.Event)["requestContext"])
	var value string
	switch r.ResponseType {
	case APIGatewayV1:
		value, _ = textValue(object(request["identity"])["sourceIp"])
	case APIGatewayV2:
		value, _ = textValue(object(request["http"])["sourceIp"])
	default:
		value, _, _ = strings.Cut(r.Request.Header.Get("X-Forwarded-For"), ",")
		value = commons.TrimSpace(value)
	}
	return value
}
