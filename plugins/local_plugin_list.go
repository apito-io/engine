package plugins

import (
	"os"

	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/engine/models"
)

var LocalPluginDir = "plugins/local"

func LoadLocalPluginRegistry(cfg *models.Config, driver *protobuff.DriverCredentials) (map[string]*protobuff.PluginDetails, error) {

	_plugin := make(map[string]*protobuff.PluginDetails)

	_aptoCDNenvVars := []*protobuff.FunctionEnvVariables{
		{
			Key:   "S3_ACCESS_KEY",
			Value: getEnv("S3_ACCESS_KEY", ""),
		},
		{
			Key:   "S3_SECRET_KEY",
			Value: getEnv("S3_SECRET_KEY", ""),
		},
		{
			Key:   "S3_CDN_URL",
			Value: getEnv("S3_CDN_URL", ".apito.io"),
		},
		{
			Key:   "S3_BUCKET_NAME",
			Value: getEnv("S3_BUCKET", "apito.io"),
		},
		{
			Key:   "S3_FOLDER",
			Value: getEnv("S3_FOLDER", "accounts"),
		},
		{
			Key:   "S3_REGION",
			Value: getEnv("S3_REGION", "ap-south-1"),
		},
	}
	if driver != nil {
		_aptoCDNenvVars = append(_aptoCDNenvVars, &protobuff.FunctionEnvVariables{
			Key:   "DB_HOST",
			Value: driver.Host,
		})
		_aptoCDNenvVars = append(_aptoCDNenvVars, &protobuff.FunctionEnvVariables{
			Key:   "DB_PORT",
			Value: driver.Port,
		})
		_aptoCDNenvVars = append(_aptoCDNenvVars, &protobuff.FunctionEnvVariables{
			Key:   "DB_USER",
			Value: driver.User,
		})
		_aptoCDNenvVars = append(_aptoCDNenvVars, &protobuff.FunctionEnvVariables{
			Key:   "DB_PASSWORD",
			Value: driver.Password,
		})
		_aptoCDNenvVars = append(_aptoCDNenvVars, &protobuff.FunctionEnvVariables{
			Key:   "DB_DATABASE",
			Value: driver.Database,
		})
	}

	_plugin["apito-cdn-file-upload"] = &protobuff.PluginDetails{
		Id:               "apito-cdn-file-upload",
		Title:            "Apito CDN",
		Icon:             "https://app.apito.io/static/pages/console/settings/extension/aws-lambda.svg",
		Description:      "Apito Media CDN Plugin ",
		Type:             protobuff.PluginType_Storage,
		EnvVars:          _aptoCDNenvVars,
		ExportedVariable: "ApitoCDNUpload",
		Enable:           true,
		Version:          "2023.07.15",
		Author:           "Apito",

		RepositoryUrl: "",
		Branch:        "",
	}

	_plugin["local-file-upload-to-storage"] = &protobuff.PluginDetails{
		Id:          "local-file-upload-to-storage",
		Title:       "Local File Upload",
		Icon:        "https://app.apito.io/static/pages/console/settings/extension/aws-lambda.svg",
		Description: "Using local file storage to store media files",
		Type:        protobuff.PluginType_Storage,
		EnvVars: []*protobuff.FunctionEnvVariables{
			{
				Key:   "UPLOAD_DIR",
				Value: "files/storage",
			},
			{
				Key:   "CDN_URL",
				Value: "https://api.apito.io",
			},
		},
		ExportedVariable: "LocalFileUpload",
		Enable:           true,
		Version:          "2023.07.15",
		Author:           "Apito",

		RepositoryUrl: "",
		Branch:        "",
	}

	_plugin["go-local-func-execute-plugin"] = &protobuff.PluginDetails{
		Id:          "go-local-func-execute-plugin",
		Title:       "Function Execution via Go Plugin",
		Icon:        "https://app.apito.io/static/pages/console/settings/extension/aws-lambda.svg",
		Description: "Using Go Plugin Method to Execute Functions",
		Type:        protobuff.PluginType_Function,
		EnvVars: []*protobuff.FunctionEnvVariables{
			{
				Key:   "FUNCTION_PATH",
				Value: "/files/functions",
			},
		},
		ExportedVariable: "LocalGoPlugin",
		Enable:           true,
		Version:          "2023.07.15",
		Author:           "Apito",

		RepositoryUrl: "",
		Branch:        "",
	}

	return _plugin, nil
}

func getEnv(key, defaults string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaults
}
