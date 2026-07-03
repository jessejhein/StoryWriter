package contextpack

import (
	"errors"
	"fmt"
)

var (
	// ErrEstimatorOverflow reports token-estimate integer overflow.
	ErrEstimatorOverflow = errors.New("estimated token overflow")
)

// Estimator counts conservative estimated input tokens for budgeting.
type Estimator interface {
	Estimate(value string) int
	Sum(values []string) int
	SumChecked(values []string) (int, error)
}

// ByteEstimator treats one UTF-8 byte as one estimated token.
type ByteEstimator struct{}

func (ByteEstimator) Estimate(value string) int {
	return len(value)
}

func (e ByteEstimator) Sum(values []string) int {
	total, err := e.SumChecked(values)
	if err != nil {
		panic(err)
	}
	return total
}

func (ByteEstimator) SumChecked(values []string) (int, error) {
	var total int64
	for _, value := range values {
		var err error
		total, err = addEstimatedTokenLength(total, len(value))
		if err != nil {
			return 0, err
		}
	}
	return int(total), nil
}

func addEstimatedTokenLength(total int64, length int) (int64, error) {
	maxInt := int64(int(^uint(0) >> 1))
	chunk := int64(length)
	if chunk < 0 || total > maxInt-chunk {
		return 0, fmt.Errorf("estimated tokens overflow: %w", ErrEstimatorOverflow)
	}
	return total + chunk, nil
}
