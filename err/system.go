package ae

import (
	"errors"
	"fmt"
	"log"
)

var LOGIN_CONFICT = errors.New("login conflict")

// ErrProjectNameTaken is returned by ApitoSystemDB.CheckProjectName when the name is already in use.
var ErrProjectNameTaken = errors.New("project name already exists")

// ErrProjectDuplicate is used when persisting a project fails due to an existing id or unique constraint (e.g. MongoDB E11000).
var ErrProjectDuplicate = errors.New("project already exists")

// core
var ProjectIdRequired = fmt.Errorf("project id required")

const NO_PROJECT_FOUND = "no project found"

var TokenIsRequired = fmt.Errorf("token is required")
var InvalidToken = fmt.Errorf("invalid token")

var NotAllowed = fmt.Errorf("you are not allowed to perform this operation")

const ROLE_IS_REQUIRED = "role is required"

var SchemaIsNil = fmt.Errorf("create a model first")

const MODEL_NAME_REQUIRED = "model name is required"

const MODEL_IS_REQUIRED = "model is required"

var ModelTypeNotFound = fmt.Errorf("model type not found")

const NEW_MODEL_NAME_REQUIRED = "new model name is required"

// third party
const S3_REGION_IS_REQUIRED = "S3 region is required"
const AWS_CREDENTIALS_ARE_NOT_SET = "AWS credentials are not set"

// #todo better exception handling and logging
func Ew(err error, context string) error {
	return fmt.Errorf("%s :: %s", context, err.Error())
}

func EwP(err error, context string) error {
	msg := fmt.Errorf("%s :: %s", context, err.Error())
	log.Println(msg)
	return msg
}

const API_CALL_LIMIT_REACHED = "api call limit reached"
const MEDIA_BANDWIDTH_LIMIT_REACHED = "media bandwidth limit reached"
const MEDIA_STORAGE_LIMIT_REACHED = "media storage limit reached"
const API_BANDWIDTH_LIMIT_REACHED = "api bandwidth limit reached"
const NUMBER_OF_RECORD_LIMIT_REACHED = "number of record limit reached"
