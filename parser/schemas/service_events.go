package schemas

import "github.com/rambow-cloud/powertools-lambda-go/parser"

var cloudFormationBase = parser.Object(field("ServiceToken", parser.String()), field("ResponseURL", absoluteURL), field("StackId", parser.String()), field("RequestId", parser.String()), field("LogicalResourceId", parser.String()), field("ResourceType", parser.String()), field("ResourceProperties", parser.Dictionary(parser.Unknown())))
var CloudFormationCustomResourceCreateSchema = cloudFormationBase.Extend(field("RequestType", parser.Literal("Create")))
var CloudFormationCustomResourceDeleteSchema = cloudFormationBase.Extend(field("RequestType", parser.Literal("Delete")), field("PhysicalResourceId", parser.String()))
var CloudFormationCustomResourceUpdateSchema = cloudFormationBase.Extend(field("RequestType", parser.Literal("Update")), field("OldResourceProperties", parser.Dictionary(parser.Unknown())), field("PhysicalResourceId", parser.String()))
var TransferFamilySchema = parser.Object(field("username", parser.String()), field("password", parser.String()), field("protocol", parser.String()), field("serverId", parser.String()), field("sourceIp", ipAddress(4)))
var ConnectOutboundCampaignsCustomerProfileSchema = parser.Object(field("ProfileId", parser.String()), field("CustomerData", parser.String()), field("IdempotencyToken", parser.String()))
var ConnectOutboundCampaignsSchema = parser.Object(
	field("InvocationMetadata", parser.Object(field("CampaignContext", parser.Object(field("CampaignId", parser.String()), field("RunId", parser.String()), field("ActionId", parser.String()), field("CampaignName", parser.String()))))),
	field("Items", parser.Object(field("CustomerProfiles", array(ConnectOutboundCampaignsCustomerProfileSchema)))),
)
