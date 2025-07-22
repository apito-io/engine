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
	User   *SystemUser `bun:"rel:belongs-to,join:user_id=user_id" bson:"user,omitempty"`
	Team   *Team       `bun:"rel:belongs-to,join:team_id=team_id" bson:"team,omitempty"`
}

type UserProject struct {
	UserID    string      `bun:"type:uuid,pk" json:"user_id" bson:"user_id,omitempty"`
	ProjectID string      `bun:"type:uuid,pk" json:"project_id" bson:"project_id,omitempty"`
	User      *SystemUser `bun:"rel:belongs-to,join:user_id=user_id" bson:"user,omitempty"`
	Project   *Project    `bun:"rel:belongs-to,join:project_id=project_id" bson:"project,omitempty"`
}

type TeamProject struct {
	TeamID    string   `bun:"type:uuid,pk" json:"team_id" bson:"team_id,omitempty"`
	ProjectID string   `bun:"type:uuid,pk" json:"project_id" bson:"project_id,omitempty"`
	Team      *Team    `bun:"rel:belongs-to,join:team_id=team_id" bson:"team,omitempty"`
	Project   *Project `bun:"rel:belongs-to,join:project_id=project_id" bson:"project,omitempty"`
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
	Teams       []*Team       `bun:"rel:has-many" json:"teams,omitempty" firestore:"teams,omitempty" bson:"teams,omitempty"`
	Users       []*SystemUser `bun:"rel:has-many" json:"users,omitempty" firestore:"users,omitempty" bson:"users,omitempty"`
}
