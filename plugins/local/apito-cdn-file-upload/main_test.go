package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/apito-io/buffers/plugins"
)

func TestName(t *testing.T) {
	obj := ApitoCDN{}

	err := obj.Init([]*plugins.EnvVariables{
		{
			Key:   "S3_CDN_URL",
			Value: "https://api.apito.io",
		},
	})

	obj.DeleteFile(context.Background(), "fahim")

	fmt.Println(err.Error())
}
