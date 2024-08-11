package main

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/apito-io/buffers/plugins"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func (s *ApitoCDN) S3Upload(ctx context.Context, file *plugins.FileDetails) (string, error) {

	key := filepath.Join(file.RemoteFilePath, file.FileName)

	uploader := s3manager.NewUploader(S3Session)
	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(s3Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(file.Buffer),
		ContentType: aws.String(file.ContentType),
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`%s/%s`, s3CdnURL, key), nil
}

func (s *ApitoCDN) S3DeleteFiles(files []*plugins.FileDetails, folder string) error {

	svc := s3.New(S3Session)
	// List Objects

	var soiList []*s3.ObjectIdentifier
	for _, file := range files {
		fmt.Printf("File to delete: %s\n", file.FileName)
		key := strings.TrimPrefix(file.FileName, s3CdnURL)
		soi := &s3.ObjectIdentifier{Key: aws.String("accounts" + key)}
		soiList = append(soiList, soi)
	}

	input := &s3.DeleteObjectsInput{
		Bucket: aws.String(s3Bucket),
		Delete: &s3.Delete{
			Objects: soiList,
			Quiet:   aws.Bool(false),
		},
	}

	_, err := svc.DeleteObjects(input)
	if err != nil {
		return err
	}

	return nil
}
