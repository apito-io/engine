package dataloader

import (
	"context"
	"fmt"
	"github.com/graph-gophers/dataloader"
)

func (s *SystemDataloader) AddressLoaderFn(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {

	handleError := func(err error) []*dataloader.Result {
		var results []*dataloader.Result
		var result dataloader.Result
		result.Error = err
		results = append(results, &result)
		return results
	}

	fmt.Println(handleError)

	return nil
}
