//go:build cloudflare

package services

func isKVKeyMissing(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "key not found" || err.Error() == "key expired"
}
