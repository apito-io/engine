package models

import (
	"encoding/json"
)

type TeamMemberAddRequest struct {
	ProjectID   string   `json:"project_id,omitempty" firestore:"project_id,omitempty" bson:"project_id,omitempty"`
	UserID      string   `json:"user_id,omitempty" firestore:"user_id,omitempty" bson:"user_id,omitempty"`
	Email       string   `json:"email,omitempty" firestore:"email,omitempty" bson:"email,omitempty"`
	Role        string   `json:"role,omitempty" firestore:"role,omitempty" bson:"role,omitempty"`
	TeamID      string   `json:"team_id,omitempty" firestore:"team_id,omitempty" bson:"team_id,omitempty"`
	Permissions []string `json:"permissions,omitempty" firestore:"permissions,omitempty" bson:"permissions,omitempty"`
}

type APIPermission struct {
	Read   string `json:"read,omitempty" firestore:"read,omitempty" bson:"read,omitempty"`
	Create string `json:"create,omitempty" firestore:"create,omitempty" bson:"create,omitempty"`
	Update string `json:"update,omitempty" firestore:"update,omitempty" bson:"update,omitempty"`
	Delete string `json:"delete,omitempty" firestore:"delete,omitempty" bson:"delete,omitempty"`
}

type Role struct {
	ID                        string                    `bun:"id,pk,notnull,type:uuid,default:gen_random_uuid()" json:"id,omitempty" firestore:"id,omitempty" bson:"_id,omitempty"`
	APIPermissions            map[string]*APIPermission `bun:",type:jsonb" json:"api_permissions,omitempty" firestore:"permissions,omitempty" bson:"api_permissions,omitempty"`
	AdministrativePermissions []string                  `json:"administrative_permissions,omitempty" firestore:"administrative_permissions,omitempty" bson:"administrative_permissions,omitempty"`
	LogicExecutions           []string                  `json:"logic_executions,omitempty" firestore:"logic_executions,omitempty" bson:"logic_executions,omitempty"`
	SystemGenerated           bool                      `json:"system_generated,omitempty" firestore:"system_generated,omitempty" bson:"system_generated,omitempty"`
	IsAdmin                   bool                      `json:"is_admin,omitempty" firestore:"is_admin,omitempty" bson:"is_admin,omitempty"`
	IsProjectUser             bool                      `json:"is_project_user,omitempty" firestore:"is_project_user,omitempty" bson:"is_project_user,omitempty"`
	ReadOnlyProject           bool                      `json:"read_only_project,omitempty" firestore:"read_only_project,omitempty" bson:"read_only_project,omitempty"`
}

// MarshalApiPermissions serializes ApiPermissions to JSON.
func (u *Role) MarshalAPIPermissions() ([]byte, error) {
	return json.Marshal(u.APIPermissions)
}

// UnmarshalApiPermissions deserializes JSON to ApiPermissions.
func (u *Role) UnmarshalAPIPermissions(data []byte) error {
	return json.Unmarshal(data, &u.APIPermissions)
}
