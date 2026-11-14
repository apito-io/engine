package storage

import "testing"

func TestPublicURLUsesCDNBaseWithoutBucket(t *testing.T) {
	s := &ProjectS3Storage{
		bucket:         "apito-free-storage",
		publicBaseURL:  "https://cdn.apito.io",
		forcePathStyle: true,
	}
	const key = "apito_website/media/01M7B9ZGZM0Q1759ZHYE8WK3X3.jpg"
	got := s.PublicURL(key)
	const want = "https://cdn.apito.io/apito_website/media/01M7B9ZGZM0Q1759ZHYE8WK3X3.jpg"
	if got != want {
		t.Fatalf("PublicURL = %s", got)
	}
}
