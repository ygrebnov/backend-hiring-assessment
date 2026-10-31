package util

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

func DecimalToCents(value string) (int64, error) {
	whole, fractional, hasDot := strings.Cut(value, ".")

	if whole == "" {
		return 0, errors.New("invalid price")
	}

	if hasDot && (len(fractional) == 0 || len(fractional) > 2) {
		return 0, errors.New("price must have at most two decimal places")
	}

	euros, err := strconv.ParseInt(whole, 10, 64)
	if err != nil || euros < 0 {
		return 0, errors.New("invalid price")
	}

	var cents int64
	if hasDot {
		fraction, err := strconv.ParseInt(fractional, 10, 64)
		if err != nil || fraction < 0 {
			return 0, errors.New("invalid price")
		}

		if len(fractional) == 1 {
			fraction *= 10
		}

		cents = fraction
	}

	if euros > (math.MaxInt64-cents)/100 {
		return 0, errors.New("price exceeds supported range")
	}

	return euros*100 + cents, nil
}
