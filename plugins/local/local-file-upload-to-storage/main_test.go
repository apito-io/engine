package main

import (
	"fmt"
	"testing"
)

func TestName(t *testing.T) {
	obj := LocalCDN{}

	err := obj.Init([]*extensions.EnvVariables{
		{
			Key:   "S3_CDN_URL",
			Value: "https://api.apito.io",
		},
	})

	fmt.Println(err.Error())
}
