package cmd

import "github.com/lcorneliussen/md365/internal/output"

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
