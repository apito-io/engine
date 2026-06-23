//go:build cloudflare

package services

import (
	"os"

	cfenv "github.com/syumai/workers/cloudflare"
)

func jwtPrivateKeyPEM() string {
	if v := cfenv.Getenv("JWT_PRIVATE_KEY"); v != "" {
		return v
	}
	return os.Getenv("JWT_PRIVATE_KEY")
}

func jwtPublicKeyPEM() string {
	if v := cfenv.Getenv("JWT_PUBLIC_KEY"); v != "" {
		return v
	}
	return os.Getenv("JWT_PUBLIC_KEY")
}
