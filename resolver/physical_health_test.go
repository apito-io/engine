package resolver

import (
	"context"
	"testing"

	"github.com/apito-io/engine/interfaces"
)

type stubLister struct {
	interfaces.ProjectDBInterface
	names []string
}

func (s *stubLister) ListTableColumnNames(ctx context.Context, tableName string) ([]string, error) {
	return s.names, nil
}

type peelWrap struct {
	interfaces.ProjectDBInterface
	inner interfaces.ProjectDBInterface
}

func (p *peelWrap) PeelProjectDB() interfaces.ProjectDBInterface {
	return p.inner
}

func TestPhysicalColumnLister_PeelsWrapper(t *testing.T) {
	inner := &stubLister{names: []string{"id", "platform"}}
	wrapped := &peelWrap{inner: inner}
	got := physicalColumnLister(wrapped)
	if got == nil {
		t.Fatal("expected lister after peel")
	}
	names, err := got.ListTableColumnNames(context.Background(), "app_release_policy")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "id" {
		t.Fatalf("unexpected names: %v", names)
	}
}

func TestPhysicalColumnLister_Direct(t *testing.T) {
	inner := &stubLister{names: []string{"id"}}
	got := physicalColumnLister(inner)
	if got == nil {
		t.Fatal("expected direct lister")
	}
}
