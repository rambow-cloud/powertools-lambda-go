// Package jsonvalue preserves numeric tokens for domain operations that require
// exact JSON numbers, while retaining JSON v2's strict input validation.
package jsonvalue

import (
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
)

// Numbers keeps the original numeric token instead of rounding it to float64.
// Non-number values use the ordinary JSON v2 decoder and its default semantics.
var Numbers = json.WithUnmarshalers(json.UnmarshalFromFunc(func(decoder *jsontext.Decoder, value *any) error {
	if decoder.PeekKind() != '0' {
		return errors.ErrUnsupported
	}
	token, err := decoder.ReadToken()
	if err != nil {
		return err
	}
	*value = jsonv1.Number(token.String())
	return nil
}))
