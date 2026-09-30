package schemas

import (
	"context"
	"net/netip"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
)

var APIGatewayCert = parser.Object(
	field("clientCertPem", parser.String()), field("subjectDN", parser.String()), field("issuerDN", parser.String()), field("serialNumber", parser.String()),
	field("validity", parser.Object(field("notBefore", parser.String()), field("notAfter", parser.String()))),
)
var APIGatewayRecord = parser.Dictionary(parser.String())
var APIGatewayStringArray = array(parser.String())
var APIGatewayHttpMethod = parser.Enum("GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS")

func ipAddress(version int) parser.Schema[any] {
	return parser.Pipe(parser.String(), parser.SchemaFunc[any](func(_ context.Context, input any) (any, []parser.Issue, error) {
		address, err := netip.ParseAddr(input.(string))
		valid := err == nil && address.Zone() == ""
		if version == 4 {
			valid = valid && address.Is4()
		} else {
			valid = valid && address.Is6()
		}
		if !valid {
			message := "Invalid IPv4 address"
			if version == 6 {
				message = "Invalid IPv6 address"
			}
			return input, []parser.Issue{{Code: "invalid_format", Message: message, Continuable: true}}, nil
		}
		return input, nil, nil
	}))
}

var gatewaySourceIP = parser.Union(ipAddress(4), ipAddress(6))
