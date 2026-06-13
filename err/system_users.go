package ae

import "errors"

// ErrProjectUsersUnsupported is returned when the active system DB driver does not implement project user CRUD.
var ErrProjectUsersUnsupported = errors.New("project users are not supported for this system database engine")
