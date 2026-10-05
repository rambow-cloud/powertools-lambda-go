package http

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

// Response is the Go equivalent of an extended proxy handler result.
// Body strings are sent verbatim; other JSON values are serialized.
type Response struct {
	StatusCode        int
	Headers           nethttp.Header
	MultiValueHeaders nethttp.Header
	Body              any
	Cookies           []string
}

// ProxyResponse is the Lambda integration response, shared by all event adapters.
type ProxyResponse struct {
	StatusCode        int                 `json:"statusCode"`
	Headers           map[string]string   `json:"headers"`
	Body              string              `json:"body"`
	IsBase64Encoded   bool                `json:"isBase64Encoded"`
	MultiValueHeaders map[string][]string `json:"multiValueHeaders,omitempty"`
	Cookies           []string            `json:"cookies,omitempty"`
	StatusDescription string              `json:"statusDescription,omitempty"`
}

func jsonBytes(value any) ([]byte, error) {
	var result bytes.Buffer
	encoder := json.NewEncoder(&result)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(result.Bytes(), []byte{'\n'}), nil
}

func ioBody(body []byte) io.ReadCloser { return io.NopCloser(bytes.NewReader(body)) }

func hasHeader(headers nethttp.Header, name string) bool {
	_, exists := headers[nethttp.CanonicalHeaderKey(name)]
	return exists
}

func readOwnedBody(body io.ReadCloser) (data []byte, err error) {
	if body == nil {
		return nil, nil
	}
	defer func() {
		if closeErr := body.Close(); err == nil {
			err = closeErr
		}
	}()
	return io.ReadAll(body)
}

func responseBody(value any) ([]byte, bool, error) {
	switch value := value.(type) {
	case nil:
		return nil, false, nil
	case string:
		return []byte(value), false, nil
	case []byte:
		return value, true, nil
	case io.Reader:
		var body []byte
		var err error
		if owned, ok := value.(io.ReadCloser); ok {
			body, err = readOwnedBody(owned)
		} else {
			body, err = io.ReadAll(value)
		}
		return body, true, err
	default:
		body, err := jsonBytes(value)
		return body, false, err
	}
}

func handlerResponse(value any, previous nethttp.Header, fallback int, streaming bool) (*nethttp.Response, bool, error) {
	headers := previous.Clone()
	if headers == nil {
		headers = make(nethttp.Header)
	}
	status, statusText := fallback, ""
	var body []byte
	var stream io.ReadCloser
	var binary bool
	var err error
	switch value := value.(type) {
	case *nethttp.Response:
		if value == nil {
			return nil, false, fmt.Errorf("response is nil")
		}
		status = value.StatusCode
		if previous == nil {
			statusText = value.Status
		}
		for name, values := range value.Header {
			if previous != nil && strings.EqualFold(name, "Set-Cookie") && len(values) > 1 {
				values = values[len(values)-1:]
			}
			headers[nethttp.CanonicalHeaderKey(name)] = append([]string(nil), values...)
		}
		if value.Body != nil {
			if streaming {
				stream = value.Body
			} else {
				body, err = readOwnedBody(value.Body)
			}
		}
	case Response:
		status = value.StatusCode
		headers.Set("Content-Type", "application/json")
		for name, values := range value.Headers {
			headers[nethttp.CanonicalHeaderKey(name)] = append([]string(nil), values...)
		}
		for name, values := range value.MultiValueHeaders {
			for _, value := range values {
				headers.Add(name, value)
			}
		}
		for _, cookie := range value.Cookies {
			headers.Add("Set-Cookie", cookie)
		}
		if reader, ok := value.Body.(io.Reader); streaming && ok {
			stream = ownedReader(reader)
		} else {
			body, _, err = responseBody(value.Body)
		}
	case []byte:
		body, binary = value, true
	case io.Reader:
		if streaming {
			stream = ownedReader(value)
		} else {
			body, _, err = responseBody(value)
		}
		binary = true
	default:
		headers.Set("Content-Type", "application/json")
		body, err = jsonBytes(value)
		if err == nil {
			proxy, ok, proxyErr := proxyResult(body)
			if proxyErr != nil {
				return nil, false, proxyErr
			}
			if ok {
				return handlerResponse(proxy, previous, fallback, streaming)
			}
		}
	}
	if err != nil {
		return nil, false, err
	}
	if status < 200 || status > 599 {
		if stream != nil {
			_ = stream.Close()
		}
		return nil, false, fmt.Errorf("response status must be between 200 and 599")
	}
	noContent := status == 204 || status == 205 || status == 304
	if (len(body) > 0 || stream != nil && stream != nethttp.NoBody) && noContent {
		if stream != nil {
			_ = stream.Close()
		}
		return nil, false, fmt.Errorf("response status %d cannot have a body", status)
	}
	if noContent {
		body = nil
	}
	var reader io.ReadCloser = nethttp.NoBody
	if body != nil {
		reader = ioBody(body)
	}
	length := int64(len(body))
	if stream != nil {
		reader, length = stream, -1
	}
	return &nethttp.Response{StatusCode: status, Status: statusText, Header: headers, Body: reader, ContentLength: length}, binary, nil
}

func ownedReader(reader io.Reader) io.ReadCloser {
	if closer, ok := reader.(io.ReadCloser); ok {
		return closer
	}
	return io.NopCloser(reader)
}

func base64Headers(headers nethttp.Header) bool {
	if encoding := headers.Get("Content-Encoding"); encoding == "gzip" || encoding == "deflate" {
		return true
	}
	media := strings.TrimSpace(strings.SplitN(headers.Get("Content-Type"), ";", 2)[0])
	return strings.HasPrefix(media, "image/") || strings.HasPrefix(media, "audio/") || strings.HasPrefix(media, "video/")
}

// HandlerResultToWebResponse converts a handler result with the default 200 status.
func HandlerResultToWebResponse(value any) (*nethttp.Response, error) {
	response, _, err := handlerResponse(value, nil, nethttp.StatusOK, false)
	return response, err
}

// WebResponseToProxyResult serializes an owned response body and closes it.
func WebResponseToProxyResult(response *nethttp.Response, kind ResponseType, encoded bool) (ProxyResponse, error) {
	result := ProxyResponse{StatusCode: response.StatusCode, Headers: map[string]string{}, IsBase64Encoded: encoded}
	var body []byte
	var err error
	if response.Body != nil {
		body, err = readOwnedBody(response.Body)
	}
	if err != nil {
		return ProxyResponse{}, err
	}
	if encoded {
		result.Body = base64.StdEncoding.EncodeToString(body)
	} else {
		result.Body = strings.TrimPrefix(commons.DecodeUTF8(body), "\ufeff")
	}
	proxyHeaders(response.Header, kind, &result)
	if kind == ALB {
		statusText := strings.TrimPrefix(response.Status, fmt.Sprintf("%d ", response.StatusCode))
		if statusText == "" {
			statusText = nethttp.StatusText(response.StatusCode)
		}
		if response.StatusCode == 418 && response.Status == "" {
			statusText = "I'm a Teapot"
		}
		if statusText == "" {
			statusText = "undefined"
		}
		result.StatusDescription = fmt.Sprintf("%d %s", response.StatusCode, statusText)
	}
	return result, nil
}

func proxyHeaders(headers nethttp.Header, kind ResponseType, result *ProxyResponse) {
	const lists = " accept accept-encoding accept-language cache-control vary connection allow x-forwarded-for te expect transfer-encoding content-encoding content-language "
	for name, values := range headers {
		name = strings.ToLower(name)
		value := strings.Join(values, ", ")
		if name == "set-cookie" || kind != APIGatewayV2 && (strings.Contains(lists, " "+name+" ") || strings.HasPrefix(name, "access-control-")) {
			parts := strings.Split(value, ",")
			for i := range parts {
				parts[i] = strings.TrimLeft(parts[i], " \t")
			}
			if name == "set-cookie" && kind == APIGatewayV2 {
				result.Cookies = append(result.Cookies, parts...)
				continue
			}
			if len(parts) > 1 {
				if result.MultiValueHeaders == nil {
					result.MultiValueHeaders = map[string][]string{}
				}
				result.MultiValueHeaders[name] = parts
				continue
			}
		}
		result.Headers[name] = value
	}
}
