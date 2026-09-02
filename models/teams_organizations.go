package models

type UserToTeams struct {
	UserID     string      `bun:"type:uuid,pk" json:"user_id,omitempty" firestore:"user_id,omitempty" bson:"user_id,omitempty"`
	SystemUser *SystemUser `bun:"rel:belongs-to,join:user_id=id" json:"system_user,omitempty" firestore:"system_user,omitempty" bson:"system_user,omitempty"`
	TeamID     string      `bun:"type:uuid,pk" json:"team_id,omitempty" firestore:"team_id,omitempty" bson:"team_id,omitempty"`
	Team       *Team       `bun:"rel:belongs-to,join:team_id=id" json:"team,omitempty" firestore:"team,omitempty" bson:"team,omitempty"`
}

type UserTeam struct {
	UserID string      `bun:"type:uuid,pk" json:"user_id" bson:"user_id,omitempty"`
	TeamID string      `bun:"type:uuid,pk" json:"team_id" bson:"team_id,omitempty"`
	User   *SystemUser `bun:"rel:belongs-to,join:user_id=id" bson:"user,omitempty"`
	Team   *Team       `bun:"rel:belongs-to,join:team_id=id" bson:"team,omitempty"`
}

type UserProject struct {
	UserID      string      `bun:"type:uuid,pk" json:"user_id" bson:"user_id,omitempty"`
	ProjectID   string      `bun:"type:uuid,pk" json:"project_id" bson:"project_id,omitempty"`
	Role        string      `bun:"role,type:text,nullzero" json:"role,omitempty" bson:"role,omitempty"`
	Permissions string      `bun:"permissions,type:text,nullzero" json:"permissions,omitempty" bson:"permissions,omitempty"`
	InviteStatus    string `bun:"invite_status,type:text,nullzero" json:"invite_status,omitempty" bson:"invite_status,omitempty"`
	InvitedAt       string `bun:"invited_at,type:text,nullzero" json:"invited_at,omitempty" bson:"invited_at,omitempty"`
	AcceptedAt      string `bun:"accepted_at,type:text,nullzero" json:"accepted_at,omitempty" bson:"accepted_at,omitempty"`
	InviteExpiresAt string `bun:"invite_expires_at,type:text,nullzero" json:"invite_expires_at,omitempty" bson:"invite_expires_at,omitempty"`
	InviteToken     string `bun:"invite_token,type:text,nullzero" json:"-" bson:"invite_token,omitempty"`
	User        *SystemUser `bun:"rel:belongs-to,join:user_id=id" bson:"user,omitempty"`
	Project     *Project    `bun:"rel:belongs-to,join:project_id=id" bson:"project,omitempty"`
}

type TeamProject struct {
	TeamID    string   `bun:"type:uuid,pk" json:"team_id" bson:"team_id,omitempty"`
	ProjectID string   `bun:"type:uuid,pk" json:"project_id" bson:"project_id,omitempty"`
	LinkedAt  string   `bun:"linked_at,type:timestamp,nullzero" json:"linked_at,omitempty" bson:"linked_at,omitempty"`
	Team      *Team    `bun:"rel:belongs-to,join:team_id=id" bson:"team,omitempty"`
	Project   *Project `bun:"rel:belongs-to,join:project_id=id" bson:"project,omitempty"`
}

// UserOrganization is the SQL join row for user ↔ organization membership (see user_organizations).
type UserOrganization struct {
	UserID         string        `bun:"type:uuid,pk" json:"user_id,omitempty" firestore:"user_id,omitempty" bson:"user_id,omitempty"`
	OrganizationID string        `bun:"type:uuid,pk" json:"organization_id,omitempty" firestore:"organization_id,omitempty" bson:"organization_id,omitempty"`
	Role           string        `bun:"role,type:text,nullzero" json:"role,omitempty" firestore:"role,omitempty" bson:"role,omitempty"`
	JoinedAt       string        `bun:"joined_at,type:timestamp,nullzero" json:"joined_at,omitempty" firestore:"joined_at,omitempty" bson:"joined_at,omitempty"`
	User           *SystemUser   `bun:"rel:belongs-to,join:user_id=id" json:"user,omitempty" firestore:"user,omitempty" bson:"user,omitempty"`
	Organization   *Organization `bun:"rel:belongs-to,join:organization_id=id" json:"organization,omitempty" firestore:"organization,omitempty" bson:"organization,omitempty"`
}

// OrganizationTeam is the SQL join row for organization ↔ team (see organization_teams).
type OrganizationTeam struct {
	OrganizationID string        `bun:"type:uuid,pk" json:"organization_id,omitempty" firestore:"organization_id,omitempty" bson:"organization_id,omitempty"`
	TeamID         string        `bun:"type:uuid,pk" json:"team_id,omitempty" firestore:"team_id,omitempty" bson:"team_id,omitempty"`
	AssignedBy     string        `bun:"assigned_by,type:uuid,nullzero" json:"assigned_by,omitempty" firestore:"assigned_by,omitempty" bson:"assigned_by,omitempty"`
	AssignedAt     string        `bun:"assigned_at,type:timestamp,nullzero" json:"assigned_at,omitempty" firestore:"assigned_at,omitempty" bson:"assigned_at,omitempty"`
	Organization   *Organization `bun:"rel:belongs-to,join:organization_id=id" json:"organization,omitempty" firestore:"organization,omitempty" bson:"organization,omitempty"`
	Team           *Team         `bun:"rel:belongs-to,join:team_id=id" json:"team,omitempty" firestore:"team,omitempty" bson:"team,omitempty"`
}

type Team struct {
	XKey        string        `json:"_key,omitempty" firestore:"_key,omitempty" bson:"_key,omitempty"`
	ID          string        `bun:"type:uuid,pk" json:"id,omitempty" firestore:"id,omitempty" bson:"_id,omitempty"`
	Name        string        `json:"name,omitempty" firestore:"name,omitempty" bson:"name,omitempty"`
	Description string        `json:"description,omitempty" firestore:"description,omitempty" bson:"description,omitempty"`
	CreatedBy   string        `json:"created_by,omitempty" firestore:"created_by,omitempty" bson:"created_by,omitempty"`
	Users       []*SystemUser `bun:"m2m:user_teams,join:Team=User" json:"users,omitempty" firestore:"users,omitempty" bson:"users,omitempty"`
	Projects    []*Project    `bun:"m2m:team_projects,join:Team=Project" json:"projects,omitempty" firestore:"projects,omitempty" bson:"projects,omitempty"`
}

type Organization struct {
	XKey        string        `json:"_key,omitempty" firestore:"_key,omitempty" bson:"_key,omitempty"`
	ID          string        `bun:"type:uuid,pk" json:"id,omitempty" firestore:"id,omitempty" bson:"_id,omitempty"`
	Name        string        `json:"name,omitempty" firestore:"name,omitempty" bson:"name,omitempty"`
	Description string        `json:"description,omitempty" firestore:"description,omitempty" bson:"description,omitempty"`
	// Owner/creator FK for SystemUser.Organization has-many (join:id=user_id).
	UserID string `bun:"user_id,type:uuid" json:"user_id,omitempty" firestore:"user_id,omitempty" bson:"user_id,omitempty"`
	Teams  []*Team       `bun:"m2m:organization_teams,join:Organization=Team" json:"teams,omitempty" firestore:"teams,omitempty" bson:"teams,omitempty"`
	Users  []*SystemUser `bun:"m2m:user_organizations,join:Organization=User" json:"users,omitempty" firestore:"users,omitempty" bson:"users,omitempty"`
}
