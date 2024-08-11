package models

type JWTTokens struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
}

type TokenClaims struct {
	UserId        string `json:"user_id"`
	Email         string `json:"email"`
	ProjectId     string `json:"project_id"`
	TokenUniqueId string `json:"token_unique_id"`
	IsProjectUser bool   `json:"is_project_user"`
	IsReadOnly    bool   `json:"is_read_only"`
}
