package billingcontract

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/shopspring/decimal"
)

const (
	NumericPrecision = 24
	NumericScale     = 12
)

var (
	ErrInvalidDecimal    = errors.New("invalid decimal")
	ErrDecimalOutOfRange = errors.New("decimal exceeds NUMERIC(24,12)")
	ErrNegativeAmount    = errors.New("amount must not be negative")
	decimalStringPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)
)

type Decimal struct {
	value decimal.Decimal
}

func ParseDecimal(raw string) (Decimal, error) {
	if raw == "" || strings.TrimSpace(raw) != raw || !decimalStringPattern.MatchString(raw) {
		return Decimal{}, ErrInvalidDecimal
	}
	parsed, err := decimal.NewFromString(raw)
	if err != nil {
		return Decimal{}, fmt.Errorf("%w: %v", ErrInvalidDecimal, err)
	}
	if parsed.Exponent() < -NumericScale || parsed.Abs().GreaterThanOrEqual(decimal.New(1, NumericPrecision-NumericScale)) {
		return Decimal{}, ErrDecimalOutOfRange
	}
	return Decimal{value: parsed}, nil
}

func (d Decimal) String() string {
	return d.value.String()
}

func (d Decimal) IsNegative() bool {
	return d.value.IsNegative()
}

func (d Decimal) Equal(other Decimal) bool {
	return d.value.Equal(other.value)
}

func (d Decimal) Add(other Decimal) Decimal {
	return Decimal{value: d.value.Add(other.value)}
}

func (d Decimal) Sub(other Decimal) Decimal {
	return Decimal{value: d.value.Sub(other.value)}
}

func (d Decimal) LessThan(other Decimal) bool {
	return d.value.LessThan(other.value)
}

func (d Decimal) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Decimal) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("%w: API decimals must be strings", ErrInvalidDecimal)
	}
	parsed, err := ParseDecimal(raw)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d Decimal) Value() (driver.Value, error) {
	return d.String(), nil
}

func (d *Decimal) Scan(src any) error {
	var raw string
	switch value := src.(type) {
	case string:
		raw = value
	case []byte:
		raw = string(value)
	default:
		return fmt.Errorf("%w: unsupported database value %T", ErrInvalidDecimal, src)
	}
	parsed, err := ParseDecimal(raw)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

type PostingRound struct {
	exact   Decimal
	posted  Decimal
	residue Decimal
}

func RoundForPosting(exact Decimal, scale int32) (PostingRound, error) {
	if scale < 0 || scale > NumericScale {
		return PostingRound{}, ErrDecimalOutOfRange
	}
	posted, err := ParseDecimal(exact.value.RoundBank(scale).String())
	if err != nil {
		return PostingRound{}, err
	}
	return PostingRound{exact: exact, posted: posted, residue: exact.Sub(posted)}, nil
}

func (r PostingRound) Exact() Decimal   { return r.exact }
func (r PostingRound) Posted() Decimal  { return r.posted }
func (r PostingRound) Residue() Decimal { return r.residue }
