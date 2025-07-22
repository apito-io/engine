package models

type SupportAndTicket struct {
	XKey             string         `json:"_key,omitempty" bson:"_key,omitempty"`
	ID               string         `json:"id,omitempty" bson:"_id,omitempty"`
	Type             string         `json:"type,omitempty" bson:"type,omitempty"`
	ProjectID        string         `json:"project_id,omitempty" bson:"project_id,omitempty"`
	Resolved         bool           `json:"resolved,omitempty" bson:"resolved,omitempty"`
	Title            string         `json:"title,omitempty" bson:"title,omitempty"`
	IssueDescription string         `json:"issue_description,omitempty" bson:"issue_description,omitempty"`
	CreatedAt        string         `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt        string         `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	Replies          []*TicketReply `json:"replies,omitempty" bson:"replies,omitempty"`
}

type TicketReply struct {
	Description string      `json:"description,omitempty" bson:"description,omitempty"`
	User        *SystemUser `json:"user,omitempty" bson:"user,omitempty"`
	CreatedAt   string      `json:"created_at,omitempty" bson:"created_at,omitempty"`
	Edited      bool        `json:"edited,omitempty" bson:"edited,omitempty"`
}
