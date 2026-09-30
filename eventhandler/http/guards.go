package http

import "encoding/json"

func eventObject(value any) wireObject {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return object(raw)
}
func IsAPIGatewayProxyEventV1(value any) bool { return isV1(eventObject(value)) }
func IsAPIGatewayProxyEventV2(value any) bool { return isV2(eventObject(value)) }
func IsALBEvent(value any) bool {
	return object(object(eventObject(value)["requestContext"])["elb"]) != nil
}
func IsExtendedAPIGatewayProxyResult(value any) bool {
	if _, ok := value.(Response); ok {
		return true
	}
	raw, err := jsonBytes(value)
	if err != nil {
		return false
	}
	_, ok, _ := proxyResult(raw)
	return ok
}
