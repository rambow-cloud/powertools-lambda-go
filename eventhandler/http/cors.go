package http

import (
	"math"
	nethttp "net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

// CORSOptions preserves the reference distinction between a single origin and
// an origin array. Origins takes precedence over Origin when non-nil. Nil lists
// use the reference defaults; non-nil empty lists disable the corresponding list.
type CORSOptions struct {
	Origin        *string
	Origins       []string
	AllowMethods  []string
	AllowHeaders  []string
	ExposeHeaders []string
	Credentials   bool
	MaxAge        *float64
}

// CORS sets response headers before downstream middleware and short-circuits
// valid OPTIONS preflight requests. Invalid preflights continue normal routing.
func CORS(options CORSOptions) Middleware {
	origins := slices.Clone(options.Origins)
	array := origins != nil
	if !array {
		origin := "*"
		if options.Origin != nil {
			origin = *options.Origin
		}
		origins = []string{origin}
	}
	wildcard := slices.Contains(origins, "*")
	methods := slices.Clone(options.AllowMethods)
	if methods == nil {
		methods = []string{"DELETE", "GET", "HEAD", "PATCH", "POST", "PUT"}
	}
	for i := range methods {
		methods[i] = strings.ToUpper(methods[i])
	}
	headers := slices.Clone(options.AllowHeaders)
	if headers == nil {
		headers = []string{"Authorization", "Content-Type", "X-Amz-Date", "X-Api-Key", "X-Amz-Security-Token"}
	}
	for i := range headers {
		headers[i] = strings.ToLower(headers[i])
	}
	exposed := slices.Clone(options.ExposeHeaders)
	credentials := options.Credentials
	maxAge, hasMaxAge := "", options.MaxAge != nil
	if hasMaxAge {
		age := *options.MaxAge
		format := byte('f')
		if magnitude := math.Abs(age); magnitude != 0 && (magnitude < 1e-6 || magnitude >= 1e21) {
			format = 'e'
		}
		maxAge = strconv.FormatFloat(age, format, -1, 64)
		maxAge = strings.ReplaceAll(maxAge, "e-0", "e-")
		maxAge = strings.ReplaceAll(maxAge, "Inf", "Infinity")
		maxAge = strings.TrimPrefix(maxAge, "+")
		if age == 0 {
			maxAge = "0"
		}
	}
	return func(request *RequestContext, next Next) error {
		origin := request.Request.Header.Get("Origin")
		if !hasHeader(request.Request.Header, "Origin") || !wildcard && !slices.Contains(origins, origin) {
			return next()
		}
		preflight := request.Request.Method == nethttp.MethodOptions
		if preflight {
			method := strings.ToUpper(request.Request.Header.Get("Access-Control-Request-Method"))
			if !hasHeader(request.Request.Header, "Access-Control-Request-Method") || !slices.Contains(methods, method) {
				return next()
			}
			if requested := strings.ToLower(request.Request.Header.Get("Access-Control-Request-Headers")); requested != "" {
				for _, header := range strings.Split(requested, ",") {
					if !slices.Contains(headers, commons.TrimSpace(header)) {
						return next()
					}
				}
			}
		}
		response := request.Response.Header
		if wildcard {
			origin = "*"
		}
		response.Set("Access-Control-Allow-Origin", origin)
		if array && !wildcard {
			response.Set("Vary", "Origin")
		}
		if credentials {
			response.Set("Access-Control-Allow-Credentials", "true")
		}
		if preflight {
			if hasMaxAge {
				response.Set("Access-Control-Max-Age", maxAge)
			}
			for _, method := range methods {
				response.Add("Access-Control-Allow-Methods", method)
			}
			for _, header := range headers {
				response.Add("Access-Control-Allow-Headers", header)
			}
			return request.Respond(&nethttp.Response{StatusCode: nethttp.StatusNoContent, Header: response})
		}
		for _, header := range exposed {
			response.Add("Access-Control-Expose-Headers", header)
		}
		return next()
	}
}
