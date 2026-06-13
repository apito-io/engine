package ae

import "errors"

// ErrProjectFilesUnsupported is returned when the active project DB driver does not implement file metadata CRUD.
var ErrProjectFilesUnsupported = errors.New("project files are not supported for this project database engine")
