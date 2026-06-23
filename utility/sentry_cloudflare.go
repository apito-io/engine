//go:build cloudflare

package utility

func CaptureInternalServerError(err error, scopes map[string]interface{}) error {
	_ = scopes
	return err
}
