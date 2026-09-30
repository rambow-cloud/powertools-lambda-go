// Package appsyncevents routes native Lambda AppSync Events invocations.
package appsyncevents

import "context"

// Event retains the original JSON envelope, including unknown identity and request fields.
// Handlers share this envelope during individual publication and must treat it as read-only.
type Event = map[string]any

type PublishHandler func(context.Context, any, Event) (any, error)
type SubscribeHandler func(context.Context, Event) error
type PublishOptions struct{ Aggregate bool }

// Undefined omits the immediate payload/events property instead of encoding JSON null.
type Undefined struct{}

func record(value any) (map[string]any, bool) {
	result, ok := value.(map[string]any)
	return result, ok && result != nil
}

func validEvent(event Event) bool {
	for _, key := range []string{"identity", "result", "error", "prev", "events"} {
		if _, present := event[key]; !present {
			return false
		}
	}
	request, ok := record(event["request"])
	if !ok {
		return false
	}
	if _, ok := record(request["headers"]); !ok {
		return false
	}
	if _, present := request["domainName"]; !present {
		return false
	}
	if _, ok := record(event["stash"]); !ok {
		return false
	}
	if _, ok := event["outErrors"].([]any); !ok {
		return false
	}
	info, ok := record(event["info"])
	if !ok {
		return false
	}
	channel, ok := record(info["channel"])
	if !ok {
		return false
	}
	if _, ok := channel["path"].(string); !ok {
		return false
	}
	segments, ok := channel["segments"].([]any)
	if !ok {
		return false
	}
	for _, segment := range segments {
		if _, ok := segment.(string); !ok {
			return false
		}
	}
	namespace, ok := record(info["channelNamespace"])
	if !ok {
		return false
	}
	if _, ok := namespace["name"].(string); !ok {
		return false
	}
	operation, _ := info["operation"].(string)
	return operation == "PUBLISH" || operation == "SUBSCRIBE"
}

func publishEvents(event Event) ([]any, bool) {
	if event["info"].(map[string]any)["operation"] != "PUBLISH" {
		return nil, false
	}
	items, ok := event["events"].([]any)
	if !ok {
		return nil, false
	}
	for _, item := range items {
		message, ok := record(item)
		if !ok {
			return nil, false
		}
		if _, ok := message["id"].(string); !ok {
			return nil, false
		}
		if _, present := message["payload"]; !present {
			return nil, false
		}
	}
	return items, true
}
