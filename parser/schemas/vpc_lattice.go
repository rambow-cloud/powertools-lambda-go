package schemas

import "github.com/rambow-cloud/powertools-lambda-go/parser"

var VpcLatticeSchema = parser.Object(field("method", APIGatewayHttpMethod), field("raw_path", parser.String()), field("body", parser.String()), field("is_base64_encoded", parser.Boolean()), field("headers", APIGatewayRecord), field("query_string_parameters", APIGatewayRecord))

// Retain legacy identity aliases alongside the AWS-documented spellings.
var latticeIdentity = parser.Object(optional("sourceVpcArn", parser.String()), optional("type", parser.String()), optional("principal", parser.String()), optional("principalOrgId", parser.String()), optional("sessionName", parser.String()), optional("X509SubjectCn", parser.String()), optional("X509IssuerOu", parser.String()), optional("x509SanDns", parser.String()), optional("x509SanUri", parser.String()), optional("X509SanNameCn", parser.String()), optional("principalOrgID", parser.String()), optional("x509SubjectCn", parser.String()), optional("x509IssuerOu", parser.String()), optional("x509SanNameCn", parser.String()))
var latticeContext = parser.Object(field("serviceNetworkArn", parser.String()), field("serviceArn", parser.String()), field("targetGroupArn", parser.String()), field("region", parser.String()), field("timeEpoch", parser.String()), field("identity", latticeIdentity))
var latticeV2Values = parser.Dictionary(APIGatewayStringArray)
var VpcLatticeV2Schema = parser.Object(field("version", parser.String()), field("path", parser.String()), field("method", APIGatewayHttpMethod), field("headers", latticeV2Values), optional("queryStringParameters", latticeV2Values), optional("body", parser.String()), optional("isBase64Encoded", parser.Boolean()), field("requestContext", latticeContext), optional("requestId", parser.String()))
