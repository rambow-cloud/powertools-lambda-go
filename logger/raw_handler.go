package logger

import (
	"context"
	jsonv1 "encoding/json"
	json "encoding/json/v2"
)

// WrapRawHandler logs the incoming JSON value before decoding it into T.
// Register the returned RawMessage function with the Lambda runtime to retain
// event members that are absent from T and values omitted by T's JSON tags.
// Decoding uses encoding/json/v2 defaults; decoding failures return without
// calling the business handler. Syntax rejected by the runtime never reaches
// this wrapper. Log formatting and replacers can still change the logged value.
// Correlation callbacks and extractors receive RawMessage, not the decoded T.
// Invocation state, logging options, and cleanup follow WrapHandler.
func WrapRawHandler[T, R any](l *Logger, handler func(context.Context, T) (R, error), options ...HandlerOptions) func(context.Context, jsonv1.RawMessage) (R, error) {
	return WrapHandler(l, func(ctx context.Context, event jsonv1.RawMessage) (R, error) {
		var input T
		if err := json.Unmarshal(event, &input); err != nil {
			var zero R
			return zero, err
		}
		return handler(ctx, input)
	}, options...)
}
