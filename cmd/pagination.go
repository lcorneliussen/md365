package cmd

import "github.com/lcorneliussen/md365/internal/output"

func collectionPage[T any](values []T, limit int) ([]T, output.ResponseOption) {
	hasMore := len(values) > limit
	if hasMore {
		values = values[:limit]
	}
	return values, output.WithCollectionPage(hasMore, "", nil)
}

func completeCollection(length int) output.ResponseOption {
	total := length
	return output.WithCollectionPage(false, "", &total)
}
