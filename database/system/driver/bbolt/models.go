package bbolt

// TeamMembership represents a user's membership in a project team
type TeamMembership struct {
	ID          string   `json:"id"`
	XKey        string   `json:"_key"`
	ProjectID   string   `json:"project_id"`
	UserID      string   `json:"user_id"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}
