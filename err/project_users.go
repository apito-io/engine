package ae

import "errors"

// ErrProjectAuthUsersUnsupported is returned when the active project DB driver does not implement app-user auth CRUD.
var ErrProjectAuthUsersUnsupported = errors.New("project auth users are not supported for this project database engine")
