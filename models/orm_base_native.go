//go:build !cloudflare

package models

import "github.com/uptrace/bun"

type ORMBase = bun.BaseModel
