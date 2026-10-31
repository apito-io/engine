package models

import "github.com/uptrace/bun"

// PaymentTransaction is a project/app-user checkout attempt. It has no tenant,
// invoice, or plan-entitlement fields. Pro links a succeeded row to tenant billing.
type PaymentTransaction struct {
	bun.BaseModel `bun:"table:payment_transactions,alias:ptx"`

	ID                      string `bun:"id,pk,type:text" json:"id" bson:"_id"`
	ProjectID               string `bun:"project_id,notnull" json:"project_id" bson:"project_id"`
	UserID                  string `bun:"user_id,nullzero" json:"user_id,omitempty" bson:"user_id,omitempty"`
	CustomerRef             string `bun:"customer_ref,nullzero" json:"customer_ref,omitempty" bson:"customer_ref,omitempty"`
	Provider                string `bun:"provider,notnull" json:"provider" bson:"provider"`
	Flow                    string `bun:"flow,notnull" json:"flow" bson:"flow"`
	ProductKey              string `bun:"product_key,nullzero" json:"product_key,omitempty" bson:"product_key,omitempty"`
	Status                  string `bun:"status,notnull" json:"status" bson:"status"`
	ProviderStatus          string `bun:"provider_status,nullzero" json:"provider_status,omitempty" bson:"provider_status,omitempty"`
	Amount                  string `bun:"amount,nullzero" json:"amount,omitempty" bson:"amount,omitempty"`
	Currency                string `bun:"currency,nullzero" json:"currency,omitempty" bson:"currency,omitempty"`
	VerifiedAmount          string `bun:"verified_amount,nullzero" json:"verified_amount,omitempty" bson:"verified_amount,omitempty"`
	VerifiedCurrency        string `bun:"verified_currency,nullzero" json:"verified_currency,omitempty" bson:"verified_currency,omitempty"`
	ClientReference         string `bun:"client_reference,notnull" json:"client_reference" bson:"client_reference"`
	ProviderSessionID       string `bun:"provider_session_id,nullzero" json:"provider_session_id,omitempty" bson:"provider_session_id,omitempty"`
	ProviderPaymentID       string `bun:"provider_payment_id,nullzero" json:"provider_payment_id,omitempty" bson:"provider_payment_id,omitempty"`
	ProviderSubscriptionID  string `bun:"provider_subscription_id,nullzero" json:"provider_subscription_id,omitempty" bson:"provider_subscription_id,omitempty"`
	IdempotencyKey          string `bun:"idempotency_key,nullzero" json:"idempotency_key,omitempty" bson:"idempotency_key,omitempty"`
	ErrorCode               string `bun:"error_code,nullzero" json:"error_code,omitempty" bson:"error_code,omitempty"`
	ErrorMessage            string `bun:"error_message,nullzero" json:"error_message,omitempty" bson:"error_message,omitempty"`
	FinancialEntity         string `bun:"financial_entity,nullzero" json:"financial_entity,omitempty" bson:"financial_entity,omitempty"`
	ProviderTransactionDate string `bun:"provider_transaction_date,nullzero" json:"provider_transaction_date,omitempty" bson:"provider_transaction_date,omitempty"`
	MetadataJSON            string `bun:"metadata_json,nullzero" json:"metadata_json,omitempty" bson:"metadata_json,omitempty"`
	CreatedAt               string `bun:"created_at,notnull" json:"created_at" bson:"created_at"`
	UpdatedAt               string `bun:"updated_at,notnull" json:"updated_at" bson:"updated_at"`
	CompletedAt             string `bun:"completed_at,nullzero" json:"completed_at,omitempty" bson:"completed_at,omitempty"`
}

// PaymentSubscription is provider recurring state for an app user. It is not a tenant plan.
type PaymentSubscription struct {
	bun.BaseModel `bun:"table:payment_subscriptions,alias:psub"`

	ID                     string `bun:"id,pk,type:text" json:"id" bson:"_id"`
	ProjectID              string `bun:"project_id,notnull" json:"project_id" bson:"project_id"`
	UserID                 string `bun:"user_id,nullzero" json:"user_id,omitempty" bson:"user_id,omitempty"`
	CustomerRef            string `bun:"customer_ref,nullzero" json:"customer_ref,omitempty" bson:"customer_ref,omitempty"`
	Provider               string `bun:"provider,notnull" json:"provider" bson:"provider"`
	ProviderSubscriptionID string `bun:"provider_subscription_id,notnull" json:"provider_subscription_id" bson:"provider_subscription_id"`
	ProviderCustomerID     string `bun:"provider_customer_id,nullzero" json:"provider_customer_id,omitempty" bson:"provider_customer_id,omitempty"`
	ProductKey             string `bun:"product_key,nullzero" json:"product_key,omitempty" bson:"product_key,omitempty"`
	Status                 string `bun:"status,notnull" json:"status" bson:"status"`
	CurrentPeriodStart     string `bun:"current_period_start,nullzero" json:"current_period_start,omitempty" bson:"current_period_start,omitempty"`
	CurrentPeriodEnd       string `bun:"current_period_end,nullzero" json:"current_period_end,omitempty" bson:"current_period_end,omitempty"`
	CancelAtPeriodEnd      bool   `bun:"cancel_at_period_end" json:"cancel_at_period_end,omitempty" bson:"cancel_at_period_end,omitempty"`
	CreatedAt              string `bun:"created_at,notnull" json:"created_at" bson:"created_at"`
	UpdatedAt              string `bun:"updated_at,notnull" json:"updated_at" bson:"updated_at"`
}

// PaymentProviderEvent dedupes provider webhook/callback event ids.
type PaymentProviderEvent struct {
	bun.BaseModel `bun:"table:payment_provider_events,alias:pev"`

	ID        string `bun:"id,pk,type:text" json:"id" bson:"_id"`
	ProjectID string `bun:"project_id,notnull" json:"project_id" bson:"project_id"`
	Provider  string `bun:"provider,notnull" json:"provider" bson:"provider"`
	EventID   string `bun:"event_id,notnull" json:"event_id" bson:"event_id"`
	CreatedAt string `bun:"created_at,notnull" json:"created_at" bson:"created_at"`
}
