package models

import (
	"fmt"

	"github.com/apito-io/buffers/protobuff"
)

type ResolverKey struct {
	Key  interface{}
	Meta *protobuff.MetaField
}

func NewResolverKey(key interface{}, meta *protobuff.MetaField) *ResolverKey {
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

func (rk *ResolverKey) GetMeta() *protobuff.MetaField {
	return rk.Meta
}
