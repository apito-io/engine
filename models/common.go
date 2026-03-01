// Package models contains common data structures and models used throughout the Apito engine
package models

import "time"

type PictureDeleteRequest struct {
	Urls      []string `json:"urls,omitempty" firestore:"urls,omitempty" bson:"urls,omitempty"`
	Model     string   `json:"model,omitempty" firestore:"model,omitempty" bson:"model,omitempty"`
	ID        string   `json:"id,omitempty" firestore:"id,omitempty" bson:"_id,omitempty"`
	FieldName string   `json:"field_name,omitempty" firestore:"field_name,omitempty" bson:"field_name,omitempty"`
}

type FileDetails struct {
	ID            string        `json:"id,omitempty" firestore:"id,omitempty" bson:"_id,omitempty"`
	XKey          string        `json:"_key,omitempty" firestore:"_key,omitempty" bson:"_key,omitempty"`
	Type          string        `json:"type,omitempty" firestore:"type,omitempty" bson:"type,omitempty"`
	FileExtension string        `json:"file_extension,omitempty" firestore:"file_extension,omitempty" bson:"file_extension,omitempty"`
	FileName      string        `json:"file_name,omitempty" firestore:"file_name,omitempty" bson:"file_name,omitempty"`
	ContentType   string        `json:"content_type,omitempty" firestore:"content_type,omitempty" bson:"content_type,omitempty"`
	Size          int64         `json:"size,omitempty" firestore:"size,omitempty" bson:"size,omitempty"`
	S3Key         string        `json:"s3_key,omitempty" firestore:"s3_key,omitempty" bson:"s3_key,omitempty"`
	URL           string        `json:"url,omitempty" firestore:"url,omitempty" bson:"url,omitempty"`
	CreatedAt     string        `json:"created_at,omitempty" firestore:"created_at,omitempty" bson:"created_at,omitempty"`
	UploadParam   *UploadParams `json:"upload_param,omitempty" firestore:"upload_param,omitempty" bson:"upload_param,omitempty"`
	Buffer        []byte        `json:"buffer,omitempty" firestore:"upload_param,omitempty" bson:"buffer,omitempty"`
}

type UploadParams struct {
	DocID      string `json:"doc_id,omitempty" firestore:"doc_id,omitempty" bson:"doc_id,omitempty"`
	ProjectID  string `json:"project_id,omitempty" firestore:"project_id,omitempty" bson:"project_id,omitempty"`
	ModelName  string `json:"model_name,omitempty" firestore:"model_name,omitempty" bson:"model_name,omitempty"`
	FieldName  string `json:"field_name,omitempty" firestore:"field_name,omitempty" bson:"field_name,omitempty"`
	AllowMulti bool   `json:"allow_multi,omitempty" firestore:"allow_multi,omitempty" bson:"allow_multi,omitempty"`
	Provider   string `json:"provider,omitempty" firestore:"provider,omitempty" bson:"provider,omitempty"`
}

type Filter struct {
	Page     uint32 `json:"page,omitempty" firestore:"page,omitempty" bson:"page,omitempty"`
	Offset   uint32 `json:"offset,omitempty" firestore:"offset,omitempty" bson:"offset,omitempty"`
	Limit    uint32 `json:"limit,omitempty" firestore:"limit,omitempty" bson:"limit,omitempty"`
	Order    string `json:"order,omitempty" firestore:"order,omitempty" bson:"order,omitempty"`
	Min      uint32 `json:"min,omitempty" firestore:"min,omitempty" bson:"min,omitempty"`
	Max      uint32 `json:"max,omitempty" firestore:"max,omitempty" bson:"max,omitempty"`
	Category string `json:"category,omitempty" firestore:"category,omitempty" bson:"category,omitempty"`
}

type Request struct {
	ID           string  `json:"id,omitempty" firestore:"id,omitempty" bson:"_id,omitempty"`
	Type         string  `json:"type,omitempty" firestore:"type,omitempty" bson:"type,omitempty"`
	Filter       *Filter `json:"filter,omitempty" firestore:"filter,omitempty" bson:"filter,omitempty"`
	SearchString string  `json:"search_string,omitempty" firestore:"search_string,omitempty" bson:"search_string,omitempty"`
	Retry        bool    `json:"retry,omitempty" firestore:"retry,omitempty" bson:"retry,omitempty"`
}

// FileLink represents a file link with metadata
type FileLink struct {
	Link      string `json:"link,omitempty" firestore:"link,omitempty" bson:"link,omitempty"`
	Title     string `json:"title,omitempty" firestore:"title,omitempty" bson:"title,omitempty"`
	CreatedAt string `json:"created_at,omitempty" firestore:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty" firestore:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

type FilePickParameter struct {
	NumberOfImages uint32      `json:"number_of_images,omitempty" firestore:"number_of_images,omitempty" bson:"number_of_images,omitempty"`
	S3Folder       string      `json:"s3_folder,omitempty" firestore:"s3_folder,omitempty" bson:"s3_folder,omitempty"`
	PickerTitle    string      `json:"picker_title,omitempty" firestore:"picker_title,omitempty" bson:"picker_title,omitempty"`
	Origin         *SystemUser `json:"origin,omitempty" firestore:"origin,omitempty" bson:"origin,omitempty"`
}

type ImageMetaInfo struct {
	Identifier string `json:"identifier,omitempty" firestore:"identifier,omitempty" bson:"identifier,omitempty"`
	Name       string `json:"name,omitempty" firestore:"name,omitempty" bson:"name,omitempty"`
	Width      uint32 `json:"width,omitempty" firestore:"width,omitempty" bson:"width,omitempty"`
	Height     uint32 `json:"height,omitempty" firestore:"height,omitempty" bson:"height,omitempty"`
	Type       string `json:"type,omitempty" firestore:"type,omitempty" bson:"type,omitempty"`
}

type RegisterRequest struct {
	User             *SystemUser `json:"user,omitempty" firestore:"user,omitempty" bson:"user,omitempty"`
	VerificationCode string      `json:"verification_code,omitempty" firestore:"profession,omitempty" bson:"verification_code,omitempty"`
	AddedByAdmin     bool        `json:"added_by_admin,omitempty" firestore:"added_by_admin,omitempty" bson:"added_by_admin,omitempty"`
}

type LoginRequest struct {
	Username string `json:"username,omitempty" firestore:"username,omitempty" bson:"username,omitempty"`
	Email    string `json:"email,omitempty" firestore:"email,omitempty" bson:"email,omitempty"`
	Secret   string `json:"secret,omitempty" firestore:"secret,omitempty" bson:"secret,omitempty"`
}

type PassChangeRequest struct {
	OldPassword string `json:"old_password,omitempty" firestore:"old_password,omitempty" bson:"old_password,omitempty"`
	NewPassword string `json:"new_password,omitempty" firestore:"new_password,omitempty" bson:"new_password,omitempty"`
}

// AdminResetPasswordRequest is the body for POST /admin/reset-password (protected by APITO_ADMIN_RESET_SECRET).
type AdminResetPasswordRequest struct {
	Email       string `json:"email"`
	NewPassword string `json:"new_password"`
	AdminSecret string `json:"admin_secret"`
}

type PreviewMode struct {
	Title  string `json:"title,omitempty" firestore:"title,omitempty" bson:"title,omitempty"`
	Icon   string `json:"icon,omitempty" firestore:"icon,omitempty" bson:"icon,omitempty"`
	Status string `json:"status,omitempty" firestore:"status,omitempty" bson:"status,omitempty"`
	ID     string `json:"id,omitempty" firestore:"id,omitempty" bson:"_id,omitempty"`
}

type InitParams struct {
	ProjectID string             `json:"project_id" bson:"project_id,omitempty"`
	ProjectDB *DriverCredentials `json:"system_credentials" bson:"system_credentials,omitempty"`
	CacheDB   *DriverCredentials `json:"cache_db" bson:"cache_db,omitempty"`
	SharedDB  *DriverCredentials `json:"shared_db" bson:"shared_db,omitempty"`
}

type Response struct {
	Message string `json:"message,omitempty" bson:"message,omitempty"`
	Code    string `json:"code,omitempty" bson:"code,omitempty"`
}

// HasScope checks if token has a specific scope
func (claims *TokenClaims) HasScope(scope string) bool {
	for _, s := range claims.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// HasAnyScope checks if token has any of the specified scopes
func (claims *TokenClaims) HasAnyScope(scopes []string) bool {
	for _, requiredScope := range scopes {
		if claims.HasScope(requiredScope) {
			return true
		}
	}
	return false
}

// HasAllScopes checks if token has all of the specified scopes
func (claims *TokenClaims) HasAllScopes(scopes []string) bool {
	for _, requiredScope := range scopes {
		if !claims.HasScope(requiredScope) {
			return false
		}
	}
	return true
}

// IsExpired checks if token is expired
func (claims *TokenClaims) IsExpired() bool {
	return time.Now().Unix() > claims.ExpireAt
}

// TimeRemaining returns seconds until token expires
func (claims *TokenClaims) TimeRemaining() int64 {
	return claims.ExpireAt - time.Now().Unix()
}
