package models

import (
	"fmt"
)

type ResolverKey struct {
	Key  interface{}
	Meta *MetaField
}

func NewResolverKey(key interface{}, meta *MetaField) *ResolverKey {
	return &ResolverKey{
		Key:  key,
		Meta: meta,
	}
}

func (rk *ResolverKey) String() string {
	return fmt.Sprintf("%v", rk.Key)
}

func (rk *ResolverKey) Raw() interface{} {
	return rk.Key
}

func (rk *ResolverKey) GetMeta() *MetaField {
	return rk.Meta
}
