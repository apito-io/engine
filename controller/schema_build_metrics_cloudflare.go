//go:build cloudflare

package controller

import (
	"context"
	"time"

	"github.com/apito-io/engine/models"
)

func recordSchemaBuildOutcome(context.Context, *models.Config, string) {}

func recordSchemaBuildDuration(context.Context, *models.Config, time.Duration, error) {}
