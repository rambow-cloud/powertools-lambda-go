package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"net/url"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

type ResponseType string

const (
	APIGatewayV1 ResponseType = "ApiGatewayV1"
	APIGatewayV2 ResponseType = "ApiGatewayV2"
	ALB          ResponseType = "ALB"
)

// RequestContext owns per-invocation state while retaining the caller's context.
type RequestContext struct {
	Context         context.Context
	Request         *nethttp.Request
	Response        *nethttp.Response
	Event           json.RawMessage
	ResponseType    ResponseType
	Route           string
	Params          map[string]string
	Store           *Store
	Shared          *Store
	IsBase64Encoded *bool
	IsHTTPStreaming bool
	Valid           ValidatedData
}

// Respond replaces the response while retaining headers already set by middleware.
func (r *RequestContext) Respond(value any) error {
	response, binary, err := handlerResponse(value, r.Response.Header, nethttp.StatusOK, r.IsHTTPStreaming)
	if err != nil {
		return err
	}
	if source, ok := value.(*nethttp.Response); r.Response.Body != nil && (!ok || source != r.Response) {
		_ = r.Response.Body.Close()
	}
	r.Response = response
	if binary {
		enabled := true
		r.IsBase64Encoded = &enabled
	}
	return nil
}

type wireObject map[string]json.RawMessage

func object(raw json.RawMessage) wireObject {
	if len(raw) == 0 || raw[0] != '{' {
		return nil
	}
	var result wireObject
	_ = json.Unmarshal(raw, &result)
	return result
}
func textValue(raw json.RawMessage) (string, bool) {
	var result string
	if len(raw) == 0 || raw[0] != '"' {
		return "", false
	}
	err := json.Unmarshal(raw, &result)
	return result, err == nil
}
func null(raw json.RawMessage) bool { return bytes.Equal(raw, []byte("null")) }
func optionalObject(raw json.RawMessage, nullable bool) bool {
	return object(raw) != nil || nullable && null(raw) || !nullable && len(raw) == 0
}
func isV2(event wireObject) bool {
	version, _ := textValue(event["version"])
	_, route := textValue(event["routeKey"])
	_, path := textValue(event["rawPath"])
	_, query := textValue(event["rawQueryString"])
	_, body := textValue(event["body"])
	cookies := event["cookies"]
	return version == "2.0" && route && path && query && object(event["headers"]) != nil && object(event["requestContext"]) != nil && isBool(event["isBase64Encoded"]) && (len(event["body"]) == 0 || body) && optionalObject(event["pathParameters"], false) && optionalObject(event["queryStringParameters"], false) && optionalObject(event["stageVariables"], false) && (len(cookies) == 0 || cookies[0] == '[')
}
func isV1(event wireObject) bool {
	_, method := textValue(event["httpMethod"])
	_, path := textValue(event["path"])
	_, resource := textValue(event["resource"])
	_, body := textValue(event["body"])
	return method && path && resource && (len(event["headers"]) == 0 || optionalObject(event["headers"], true)) && (len(event["multiValueHeaders"]) == 0 || optionalObject(event["multiValueHeaders"], true)) && object(event["requestContext"]) != nil && isBool(event["isBase64Encoded"]) && (null(event["body"]) || body) && optionalObject(event["pathParameters"], true) && optionalObject(event["queryStringParameters"], true) && optionalObject(event["multiValueQueryStringParameters"], true) && optionalObject(event["stageVariables"], true)
}
func isBool(raw json.RawMessage) bool { return string(raw) == "true" || string(raw) == "false" }
func IsHTTPMethod(method string) bool {
	return strings.Contains(" GET POST PUT PATCH DELETE HEAD OPTIONS ", " "+method+" ")
}

// Each object is traversed in JSON order, preserving raw-event query value order.
func visitObject(raw json.RawMessage, visit func(string, json.RawMessage) error) error {
	if len(raw) == 0 || null(raw) {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return fmt.Errorf("expected an object")
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
		if err := visit(token.(string), value); err != nil {
			return err
		}
	}
	_, err := decoder.Token()
	return err
}

func headerValue(raw json.RawMessage) (string, error) {
	if value, ok := textValue(raw); ok {
		return strings.Trim(value, " \t\r\n"), nil
	}
	if null(raw) {
		return "null", nil
	}
	if isBool(raw) || len(raw) > 0 && (raw[0] == '-' || raw[0] >= '0' && raw[0] <= '9') {
		return string(raw), nil
	}
	return "", fmt.Errorf("header value must be a scalar")
}
func setHeader(headers nethttp.Header, name, value string, appendValue bool) error {
	if name == "" || strings.ContainsAny(name, " ()<>@,;:\\\"/[]?={}\t\r\n") || strings.ContainsAny(value, "\r\n\x00") {
		return fmt.Errorf("invalid HTTP header")
	}
	if appendValue && headers.Get(name) != "" {
		value = headers.Get(name) + ", " + value
	}
	headers.Set(name, value)
	return nil
}

func eventRequest(ctx context.Context, input any) (*nethttp.Request, ResponseType, json.RawMessage, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, "", nil, err
	}
	event := object(raw)
	kind := APIGatewayV1
	requestContext := object(event["requestContext"])
	if isV2(event) {
		kind = APIGatewayV2
	} else if object(requestContext["elb"]) != nil {
		kind = ALB
	} else if !isV1(event) {
		return nil, "", raw, &InvalidEventError{}
	}
	method, _ := textValue(event["httpMethod"])
	path, _ := textValue(event["path"])
	if kind == APIGatewayV2 {
		method, _ = textValue(object(requestContext["http"])["method"])
		path, _ = textValue(event["rawPath"])
	}
	if !IsHTTPMethod(strings.ToUpper(method)) {
		return nil, kind, raw, &InvalidHTTPMethodError{strings.ToUpper(method)}
	}
	headers := make(nethttp.Header)
	err = visitObject(event["headers"], func(name string, raw json.RawMessage) error {
		value, err := headerValue(raw)
		if err != nil {
			return err
		}
		return setHeader(headers, name, value, false)
	})
	if err != nil {
		return nil, kind, raw, err
	}
	if kind == APIGatewayV2 {
		if cookies := event["cookies"]; len(cookies) > 0 {
			var values []string
			if err := json.Unmarshal(cookies, &values); err != nil {
				return nil, kind, raw, err
			}
			if err := setHeader(headers, "Cookie", strings.Join(values, "; "), false); err != nil {
				return nil, kind, raw, err
			}
		}
	} else {
		err = visitObject(event["multiValueHeaders"], func(name string, raw json.RawMessage) error {
			var values []string
			if err := json.Unmarshal(raw, &values); err != nil {
				return err
			}
			for _, value := range values {
				if !strings.Contains(headers.Get(name), value) || headers.Get(name) == "" {
					if err := setHeader(headers, name, strings.Trim(value, " \t\r\n"), true); err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err != nil {
			return nil, kind, raw, err
		}
	}
	host := headers.Get("Host")
	if host == "" {
		host, _ = textValue(requestContext["domainName"])
		if host == "" {
			if kind == ALB {
				host = "localhost"
			} else {
				host = "undefined"
			}
		}
	}
	protocol := headers.Get("X-Forwarded-Proto")
	if protocol == "" {
		protocol = "https"
	}
	base, err := url.Parse(protocol + "://" + host + "/")
	if err != nil {
		return nil, kind, raw, err
	}
	var target *url.URL
	if kind == APIGatewayV2 {
		query, _ := textValue(event["rawQueryString"])
		address := protocol + "://" + host + path
		if query != "" {
			address += "?" + query
		}
		target, err = url.Parse(address)
	} else {
		var relative *url.URL
		relative, err = url.Parse(path)
		if err == nil {
			target = base.ResolveReference(relative)
		}
	}
	if err != nil {
		return nil, kind, raw, err
	}
	if target.User != nil || target.Host == "" {
		return nil, kind, raw, fmt.Errorf("invalid HTTP request URL")
	}
	if kind != APIGatewayV2 {
		multi := object(event["multiValueQueryStringParameters"])
		appendQuery := func(name, value string) {
			if target.RawQuery != "" {
				target.RawQuery += "&"
			}
			target.RawQuery += url.QueryEscape(name) + "=" + url.QueryEscape(value)
		}
		err = visitObject(event["queryStringParameters"], func(name string, raw json.RawMessage) error {
			if _, present := multi[name]; present && !null(multi[name]) {
				return nil
			}
			if null(raw) {
				return nil
			}
			value, ok := textValue(raw)
			if !ok {
				return fmt.Errorf("query parameter must be a string")
			}
			appendQuery(name, value)
			return nil
		})
		if err == nil {
			err = visitObject(event["multiValueQueryStringParameters"], func(name string, raw json.RawMessage) error {
				var values []string
				if err := json.Unmarshal(raw, &values); err != nil {
					return err
				}
				for _, value := range values {
					appendQuery(name, value)
				}
				return nil
			})
		}
		if err != nil {
			return nil, kind, raw, err
		}
	}
	body, hasBody := textValue(event["body"])
	if kind == ALB && (method == "GET" || method == "HEAD") {
		hasBody = false
	}
	// Fetch normalizes these six methods, but preserves PATCH's original casing.
	upper := strings.ToUpper(method)
	if upper != "PATCH" {
		method = upper
	}
	if hasBody && (method == "GET" || method == "HEAD") {
		return nil, kind, raw, fmt.Errorf("Request with GET/HEAD method cannot have body.")
	}
	if hasBody && string(event["isBase64Encoded"]) == "true" {
		body = commons.DecodeUTF8(commons.DecodeBase64Buffer(body))
	}
	request, err := nethttp.NewRequestWithContext(ctx, method, target.String(), nil)
	if err != nil {
		return nil, kind, raw, err
	}
	if hasBody {
		request.Body = ioBody([]byte(body))
		request.ContentLength = int64(len(body))
		request.GetBody = func() (io.ReadCloser, error) { return ioBody([]byte(body)), nil }
		if headers.Get("Content-Type") == "" {
			headers.Set("Content-Type", "text/plain;charset=UTF-8")
		}
	}
	request.Header = headers
	if request.Body == nil {
		request.Body = nethttp.NoBody
	}
	return request, kind, raw, nil
}

// ProxyEventToWebRequest adapts JSON values or typed Lambda event structures.
func ProxyEventToWebRequest(ctx context.Context, event any) (*nethttp.Request, ResponseType, error) {
	request, kind, _, err := eventRequest(ctx, event)
	return request, kind, conversionError(err)
}
