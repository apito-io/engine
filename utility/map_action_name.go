package utility

func IsInActionNameMap(key string) bool {
	_, ok := MapActionName[key]
	return ok
}

var MapActionName = map[string]string{
	"UpdateProfileResolverFn":        "update user profile",
	"UpsertPluginResolverFn":         "plugin modification",
	"GenerateApiTokenResolverFn":     "generated api token",
	"DeleteApiTokenResolverFn":       "deleted api token",
	"CreateWebHookResolverFn":        "created a webhook",
	"DeleteWebHookResolverFn":        "deleted a webhook",
	"UpdateProjectResolverFn":        "updated a project information",
	"AddModelToProjectResolverFn":    "added a model to a project",
	"UpdateModelResolverFn":          "updated a model",
	"UpsertFieldToModelResolverFn":   "added or modified a field to a model", // need to be separated
	"DeleteFieldTypeResolverFn":      "deleted a field",
	"ModelFieldOperationResolverFn":  "modified a field of a model",
	"CreateConnectionTypeResolverFn": "created a connection between models",
	"UpsertModelDataFnFn":            "added or modified data of a model",
	"DeleteModelDataFnFn":            "deleted data of a model",
}
