package models


type ValidIdentifier struct {
	Label string
	Identifier string
	ParentField string
}

type AuditLogs struct {
	XKey      string `json:"_key,omitempty" firestore:"_key,omitempty" bson:"_key,omitempty"`
	ID        string `json:"id,omitempty" firestore:"id,omitempty" bson:"_id,omitempty"`
	UserID    string `json:"user_id,omitempty" firestore:"user_id,omitempty" bson:"user_id,omitempty"`
	ProjectID string `json:"project_id,omitempty" firestore:"project_id,omitempty" bson:"project_id,omitempty"`

	RequestPayload  string `json:"request_payload,omitempty" firestore:"request_payload,omitempty" bson:"request_payload,omitempty"`
	RequestPath     string `json:"request_path,omitempty" firestore:"request_path,omitempty" bson:"request_path,omitempty"`
	ResponseCode    int    `json:"response_code,omitempty" firestore:"response_code,omitempty" bson:"response_code,omitempty"`
	ResponsePayload string `json:"response_payload,omitempty" firestore:"response_payload,omitempty" bson:"response_payload,omitempty"`

	Activity         string `json:"activity,omitempty" firestore:"activity,omitempty" bson:"activity,omitempty"`
	InternalFunction string `json:"internal_function,omitempty" firestore:"internal_function,omitempty" bson:"internal_function,omitempty"`

	GraphqlOperationName string `json:"graphql_operation_name,omitempty" firestore:"graphql_operation_name,omitempty" bson:"graphql_operation_name,omitempty"`
	GraphqlPayload       string `json:"graphql_payload,omitempty" firestore:"graphql_payload,omitempty" bson:"graphql_payload,omitempty"`
	GraphqlVariable      string `json:"graphql_variable,omitempty" firestore:"graphql_variable,omitempty" bson:"graphql_variable,omitempty"`

	InternalError string `json:"internal_error,omitempty" firestore:"internal_error,omitempty" bson:"internal_error,omitempty"`
	CreatedAt     string `json:"created_at,omitempty" firestore:"created_at,omitempty" bson:"created_at,omitempty"`
}

type DriverCredentials struct {
	// for sql migration purposes
	ProjectID string `bun:"type:uuid,pk" json:"project_id,omitempty" firestore:"project_id,omitempty" bson:"project_id,omitempty"`
	// normal sql & nosql database
	Engine   string `json:"engine,omitempty" bson:"engine,omitempty"`
	Host     string `json:"host,omitempty" bson:"host,omitempty"`
	Port     string `json:"port,omitempty" bson:"port,omitempty"`
	User     string `json:"user,omitempty" bson:"user,omitempty"`
	Password string `json:"password,omitempty" bson:"password,omitempty"`
	Database string `json:"database,omitempty" bson:"database,omitempty"`
	// for embedded database
	DatabaseDir string `json:"database_dir,omitempty" bson:"database_dir,omitempty"`
	// for sqlite and other file based databases
	File string `json:"file,omitempty" bson:"file,omitempty"`
	// SSLMode for PostgreSQL when the server requires TLS. Empty defaults to disable in DSN builder.
	SSLMode string `json:"ssl_mode,omitempty" bson:"ssl_mode,omitempty"`
}

type ProjectToken struct {
	ProjectID string `json:"project_id,omitempty" firestore:"project_id,omitempty" bson:"project_id,omitempty"`
	Name      string `json:"name,omitempty" firestore:"name,omitempty" bson:"name,omitempty"`
	Token     string `json:"token,omitempty" firestore:"token,omitempty" bson:"token,omitempty"`
	Role      string `json:"role,omitempty" firestore:"role,omitempty" bson:"role,omitempty"`
	Expire    string `json:"expire,omitempty" firestore:"expire,omitempty" bson:"expire,omitempty"`
}

type SyncToken struct {
	ProjectID string `json:"project_id,omitempty" firestore:"project_id,omitempty" bson:"project_id,omitempty"`
	Name      string `json:"name,omitempty" firestore:"name,omitempty" bson:"name,omitempty"`
	Token     string `json:"token,omitempty" firestore:"token,omitempty" bson:"token,omitempty"`
	Expire    string `json:"expire,omitempty" firestore:"expire,omitempty" bson:"expire,omitempty"`
	CreatedAt string `json:"created_at,omitempty" firestore:"created_at,omitempty" bson:"created_at,omitempty"`
	// extra information
	ProjectIDs []string `json:"project_ids,omitempty" firestore:"project_ids,omitempty" bson:"project_ids,omitempty"`
	Scopes     []string `json:"scopes,omitempty" firestore:"scopes,omitempty" bson:"scopes,omitempty"`
}

type ProjectCreateRequest struct {
	ID          string             `json:"id" bson:"_id,omitempty"`
	Name        string             `json:"name" bson:"name,omitempty"`
	Description string             `json:"description" bson:"description,omitempty"`
	Token       string             `json:"token" bson:"token,omitempty"`
	Engine      string             `json:"engine" bson:"engine,omitempty"`
	Driver      *DriverCredentials `json:"driver" bson:"driver,omitempty"`
	Example     string             `json:"example" bson:"example,omitempty"`
}

type Workspace struct {
	ProjectID    string `json:"project_id,omitempty" firestore:"project_id,omitempty" bson:"project_id,omitempty"`
	Name         string `json:"name,omitempty" firestore:"name,omitempty" bson:"name,omitempty"`
	Active       bool   `json:"active,omitempty" firestore:"active,omitempty" bson:"active,omitempty"`
	IsProduction bool   `json:"is_production,omitempty" firestore:"is_production,omitempty" bson:"is_production,omitempty"`
	IsDefault    bool   `json:"is_default,omitempty" firestore:"is_default,omitempty" bson:"is_default,omitempty"`
}

type SystemMessage struct {
	ProjectID   string `json:"project_id,omitempty" firestore:"project_id,omitempty" bson:"project_id,omitempty"`
	Message     string `json:"message,omitempty" firestore:"message,omitempty" bson:"message,omitempty"`
	Code        string `json:"code,omitempty" firestore:"code,omitempty" bson:"code,omitempty"`
	Redirection string `json:"redirection,omitempty" firestore:"redirection,omitempty" bson:"redirection,omitempty"`
	Hide        bool   `json:"hide,omitempty" firestore:"hide,omitempty" bson:"hide,omitempty"`
}

type Webhook struct {
	ID              string   `json:"id,omitempty" firestore:"id,omitempty" bson:"_id,omitempty"`
	XKey            string   `json:"_key,omitempty" firestore:"_key,omitempty" bson:"_key,omitempty"`
	Type            string   `json:"type,omitempty" firestore:"type,omitempty" bson:"type,omitempty"`
	Model           string   `json:"model,omitempty" firestore:"model,omitempty" bson:"model,omitempty"`
	ProjectID       string   `json:"project_id,omitempty" firestore:"project_id,omitempty" bson:"project_id,omitempty"` //
	Name            string   `json:"name,omitempty" firestore:"name,omitempty" bson:"name,omitempty"`
	Events          []string `json:"events,omitempty" firestore:"events,omitempty" bson:"events,omitempty"`
	URL             string   `json:"url,omitempty" firestore:"url,omitempty" bson:"url,omitempty"`
	LogicExecutions []string `json:"logic_executions,omitempty" firestore:"logic_executions,omitempty" bson:"logic_executions,omitempty"`
}
