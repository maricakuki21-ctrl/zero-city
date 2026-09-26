package repository

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/shopspring/decimal"
)

// sharedPoolJSONBText keeps JSON parameters on lib/pq's text path. In
// particular, an empty json.RawMessage is a non-NULL, zero-length []byte
// parameter, which PostgreSQL cannot parse as JSONB.
func sharedPoolJSONBText(raw json.RawMessage, field string) (string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return `{}`, nil
	}
	if !json.Valid(raw) {
		return "", fmt.Errorf("%s must be valid JSON", field)
	}
	return string(raw), nil
}

// sharedPoolJSONBEqual mirrors JSONB equality for the generated price
// snapshots used by shared-pool billing. Object order, whitespace, and numeric
// spelling are not material, while every decoded value must still match.
func sharedPoolJSONBEqual(left, right []byte) bool {
	var leftValue any
	var rightValue any
	if !decodeSharedPoolJSONNumber(left, &leftValue) || !decodeSharedPoolJSONNumber(right, &rightValue) {
		return false
	}
	return equalSharedPoolJSONValue(leftValue, rightValue)
}

func decodeSharedPoolJSONNumber(raw []byte, value *any) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return decoder.Decode(value) == nil
}

func equalSharedPoolJSONValue(left, right any) bool {
	switch leftValue := left.(type) {
	case json.Number:
		rightValue, ok := right.(json.Number)
		if !ok {
			return false
		}
		leftDecimal, leftErr := decimal.NewFromString(string(leftValue))
		rightDecimal, rightErr := decimal.NewFromString(string(rightValue))
		return leftErr == nil && rightErr == nil && leftDecimal.Equal(rightDecimal)
	case []any:
		rightValue, ok := right.([]any)
		if !ok || len(leftValue) != len(rightValue) {
			return false
		}
		for i := range leftValue {
			if !equalSharedPoolJSONValue(leftValue[i], rightValue[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		rightValue, ok := right.(map[string]any)
		if !ok || len(leftValue) != len(rightValue) {
			return false
		}
		for key, value := range leftValue {
			rightItem, exists := rightValue[key]
			if !exists || !equalSharedPoolJSONValue(value, rightItem) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(left, right)
	}
}
