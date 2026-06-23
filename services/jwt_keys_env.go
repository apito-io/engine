//go:build !cloudflare

package services

import "os"

func jwtPrivateKeyPEM() string {
	return os.Getenv("JWT_PRIVATE_KEY")
}

func jwtPublicKeyPEM() string {
	return os.Getenv("JWT_PUBLIC_KEY")
}
