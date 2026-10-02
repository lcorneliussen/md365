package cmd

import (
	"math"

	"github.com/lcorneliussen/md365/internal/output"
)

func lookaheadLimit(limit int) (int, error) {
	if limit <= 0 {
		return 0, usageError("--limit must be greater than zero")
	}
	if limit == math.MaxInt {
		return 0, usageError("--limit is too large")
	}
	return limit + 1, nil
}

func collectionPage[T any](values []T, limit int) ([]T, output.ResponseOption) {
	hasMore := len(values) > limit
	if hasMore {
		values = values[:limit]
	}
	if hasMore {
		return values, output.WithCollectionPage(true, "", nil)
	}
	total := len(values)
	return values, output.WithCollectionPage(false, "", &total)
}

func completeCollection(length int) output.ResponseOption {
	total := length
	return output.WithCollectionPage(false, "", &total)
}
