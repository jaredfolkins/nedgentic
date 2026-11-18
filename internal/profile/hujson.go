package profile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/tailscale/hujson"
)

func UnmarshalHuJSON(data []byte, v any) error {
	standard, err := standardizeHuJSON(data)
	if err != nil {
		return fmt.Errorf("invalid hujson: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(standard))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}
	if err := ensureDecoderEOF(decoder); err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}
	return nil
}

func standardizeHuJSON(data []byte) ([]byte, error) {
	ast, err := hujson.Parse(data)
	if err != nil {
		return nil, err
	}
	ast.Standardize()
	return ast.Pack(), nil
}

func ensureDecoderEOF(decoder *json.Decoder) error {
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing data")
		}
		return err
	}
	return nil
}
