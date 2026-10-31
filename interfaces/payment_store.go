package interfaces

import (
	"context"

	"github.com/apito-io/engine/models"
)

// PaymentListFilter scopes ledger reads to one project.
type PaymentListFilter struct {
	ProjectID string
	Provider  string
	Status    string
	UserID    string
	Query     string
	Limit     int
	Offset    int
}

// PaymentStore is the OSS payment ledger. Drivers that cannot persist it must not implement this.
type PaymentStore interface {
	UpsertPaymentTransaction(ctx context.Context, row *models.PaymentTransaction) error
	GetPaymentTransaction(ctx context.Context, projectID, id string) (*models.PaymentTransaction, error)
	FindPaymentTransactionByClientRef(ctx context.Context, projectID, provider, clientRef string) (*models.PaymentTransaction, error)
	FindPaymentTransactionByIdempotency(ctx context.Context, projectID, key string) (*models.PaymentTransaction, error)
	ListPaymentTransactions(ctx context.Context, filter PaymentListFilter) ([]*models.PaymentTransaction, error)
	UpsertPaymentSubscription(ctx context.Context, row *models.PaymentSubscription) error
	GetPaymentSubscription(ctx context.Context, projectID, id string) (*models.PaymentSubscription, error)
	FindPaymentSubscriptionByProviderRef(ctx context.Context, projectID, provider, providerSubscriptionID string) (*models.PaymentSubscription, error)
	InsertPaymentProviderEvent(ctx context.Context, row *models.PaymentProviderEvent) (bool, error)
	DeletePaymentProviderEvent(ctx context.Context, projectID, provider, eventID string) error
}
