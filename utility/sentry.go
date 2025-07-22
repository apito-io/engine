package utility

import (
	"github.com/getsentry/sentry-go"
	"time"
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
