//go:build !cloudflare

package utility

import (
	"time"

	"github.com/getsentry/sentry-go"
)

func CaptureInternalServerError(err error, scopes map[string]interface{}) error {
	sentry.WithScope(func(scope *sentry.Scope) {
		if len(scopes) > 0 {
			for k, v := range scopes {
				scope.SetExtra(k, v)
			}
		}
		sentry.CaptureException(err)
	})
	sentry.Flush(time.Second * 2)
	return err
}
