package billingcontract

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecimalRoundTripsAPIAndNumericWithoutFloatCoercion(t *testing.T) {
	// Given
	input := []byte(`"0.0001"`)

	// When
	var apiValue Decimal
	require.NoError(t, json.Unmarshal(input, &apiValue))
	databaseValue, err := apiValue.Value()
	require.NoError(t, err)
	var restored Decimal
	require.NoError(t, restored.Scan(databaseValue))
	output, err := json.Marshal(restored)

	// Then
	require.NoError(t, err)
	require.Equal(t, input, output)
}

func TestDecimalRejectsScaleBeyondNumericContract(t *testing.T) {
	// Given / When
	_, err := ParseDecimal("0.0000000000001")

	// Then
	require.ErrorIs(t, err, ErrDecimalOutOfRange)
}

func TestRoundForPostingMakesResidueExplicit(t *testing.T) {
	// Given
	exact, err := ParseDecimal("0.00015")
	require.NoError(t, err)

	// When
	result, err := RoundForPosting(exact, 4)

	// Then
	require.NoError(t, err)
	require.Equal(t, "0.0002", result.Posted().String())
	require.Equal(t, "-0.00005", result.Residue().String())
	require.True(t, result.Posted().Add(result.Residue()).Equal(result.Exact()))
}

func TestDecimalAPIRejectsJSONNumber(t *testing.T) {
	// Given / When
	var value Decimal
	err := json.Unmarshal([]byte(`0.0001`), &value)

	// Then
	require.True(t, errors.Is(err, ErrInvalidDecimal))
}
