package resolver

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/apito-io/buffers/protobuff"
	_const "github.com/apito-io/databasedriver"
	"github.com/apito-io/databasedriver/project/driver/sql"
	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/plugins"
	"github.com/apito-io/engine/utility"
	"github.com/iancoleman/strcase"
	"github.com/jinzhu/inflection"
	"github.com/labstack/echo/v4"
	"github.com/tailor-inc/graphql"
)

func (s *GraphQLServer) UpdateProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	var projectId string
	if val, ok := p.Args["_id"].(string); ok && val != "" {
		projectId = val

		project, err := s.SystemDriver.GetProject(p.Context, param.ProjectId)
		if err != nil {
			return nil, err
		}

		if project == nil {
			return nil, errors.New("this is not your project")
		}

	} else {
		// passing the current
		projectId = param.ProjectId
	}

	project, err := s.SystemDriver.GetProject(p.Context, projectId)
	if err != nil {
		return nil, err
	}

	// update project
	if val, ok := p.Args["name"].(string); ok {
		project.Name = val
	}

	if val, ok := p.Args["description"].(string); ok {
		project.Description = val
	}

	if driver, ok := p.Args["driver"].(map[string]interface{}); ok {

		if project.Driver == nil {
			project.Driver = &protobuff.DriverCredentials{Engine: "apito"}
		}

		var host string
		if val, ok := driver["host"].(string); ok {
			host = val
		}

		var port string
		if val, ok := driver["port"].(string); ok {
			port = val
		}

		var database string
		if val, ok := driver["database"].(string); ok {
			database = val
		}

		var user string
		if val, ok := driver["user"].(string); ok {
			user = val
		}

		var password string
		if val, ok := driver["password"].(string); ok {
			password = val
		}

		var db interface{}
		var err error
		switch driver["engine"] {
		case _const.SQLiteDriver, _const.MySQLDriver, _const.PostgresSQLDriver, _const.SQLServerDriver:
			db, err = sql.GetSQLDriver(&protobuff.DriverCredentials{
				Host:     host,
				Port:     port,
				Database: database,
				User:     user,
				Password: password,
			})
		case _const.DynamoDB:

		default:
			project.Driver = &protobuff.DriverCredentials{Engine: "apito"}
		}

		if db == nil {
			return nil, errors.New("db configuration is not correct")
		}

		if err != nil {
			return nil, err
		}
	}

	if vals, ok := p.Args["locals"].([]interface{}); ok {
		// #todo if purchased locals then give them locals
		for _, l := range vals {
			if !utility.ArrayContains(project.Locals, l.(string)) && utility.ArrayContains(utility.SupportedLocals, l.(string)) {
				project.Locals = append(project.Locals, l.(string))
			}
		}
	}

	if plugins, ok := p.Args["plugins"].(map[string]interface{}); ok {

		if project.Plugins == nil {
			project.Plugins = []*protobuff.PluginDetails{}
		}

		switch plugins["name"] {
		case "aws":
			details := &protobuff.PluginDetails{
				Id:          "aws",
				Description: "Aws Lambda Functions",
				EnvVars:     []*protobuff.FunctionEnvVariables{},
			}
			if val, ok := plugins["details"].(map[string]interface{}); ok {
				if accessKey, ok := val["access_key"].(string); ok {
					details.EnvVars = append(details.EnvVars, &protobuff.FunctionEnvVariables{
						Key:   "ACCESS_KEY",
						Value: accessKey,
					})
				}
				if secretKey, ok := val["secret_key"].(string); ok {
					details.EnvVars = append(details.EnvVars, &protobuff.FunctionEnvVariables{
						Key:   "SECRET_KEY",
						Value: secretKey,
					})
				}
				if region, ok := val["region"].(string); ok {
					details.EnvVars = append(details.EnvVars, &protobuff.FunctionEnvVariables{
						Key:   "RELIGION",
						Value: region,
					})
				}
			}

			/*// validate the creds first
			sess, err := session.NewSession(&aws.Config{
				Region:      aws.String(details.Configs.Credentials.Region),
				Credentials: credentials.NewStaticCredentials(details.Configs.Credentials.AccessKey, details.Configs.Credentials.SecretKey, ""),
			})
			if err != nil {
				return nil, err
			}
			_, err = sess.Config.Credentials.Get()
			if err != nil {
				return nil, err
			}

			svc := iam.New(sess)

			arn := "arn:aws:iam::aws:policy/AWSLambdaExecute"
			result, err := svc.GetPolicy(&iam.GetPolicyInput{
				PolicyArn: &arn,
			})
			if err != nil {
				return nil, err
			}

			fmt.Printf("%s - %s\n", arn, *result.Policy.Description)*/

			project.Plugins = append(project.Plugins, details)
			break
		case "apitofunc":

			details := &protobuff.PluginDetails{
				Id:          "apitofunc",
				Description: "Apito Functions",
				EnvVars:     []*protobuff.FunctionEnvVariables{},
			}
			if val, ok := plugins["details"].(map[string]interface{}); ok {
				if accessKey, ok := val["access_key"].(string); ok {
					details.EnvVars = append(details.EnvVars, &protobuff.FunctionEnvVariables{
						Key:   "ACCESS_KEY",
						Value: accessKey,
					})
				}
				if secretKey, ok := val["secret_key"].(string); ok {
					details.EnvVars = append(details.EnvVars, &protobuff.FunctionEnvVariables{
						Key:   "SECRET_KEY",
						Value: secretKey,
					})
				}
				if region, ok := val["region"].(string); ok {
					details.EnvVars = append(details.EnvVars, &protobuff.FunctionEnvVariables{
						Key:   "RELIGION",
						Value: region,
					})
				}
			}

			out, err := exec.Command("bash", "-c", "docker version").Output()

			// if there is an error with our execution
			// handle it here
			if err != nil {
				fmt.Println(err.Error())
			}
			fmt.Println("Command Successfully Executed")
			output := strings.Split(string(out), "\n")
			fmt.Println(output)
			var isDockerRunning bool
			for _, o := range output {
				if strings.TrimSpace(o) == "Engine:" {
					isDockerRunning = true
					break
				}
			}

			if isDockerRunning {
				project.Plugins = append(project.Plugins, details)
			} else {
				return nil, errors.New("docker Service is Not Running on this Machine. Please Start")
			}
		default:
			return nil, errors.New("invalid Extension Type")
		}
	}

	err = s.SystemDriver.UpdateProject(p.Context, project, true)
	if err != nil {
		return nil, err
	}

	// hide the schema
	project.Schema = nil
	return project, nil
}

func (s *GraphQLServer) UpdateProfileResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
		ctx    = p.Context
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	userId := param.UserId
	user, err := s.SystemDriver.GetSystemUser(ctx, userId)
	if err != nil {
		return nil, err
	}

	// update project
	if val, ok := p.Args["first_name"].(string); ok {
		user.FirstName = val
		if val == "fahim" {
			return nil, errors.New("fahim is a reserved word")
		}
	}

	if val, ok := p.Args["last_name"].(string); ok {
		user.LastName = val
	}

	if val, ok := p.Args["username"].(string); ok {
		user.Username = val
	}

	if val, ok := p.Args["old_pass"].(string); ok {
		if newPass, ok := p.Args["new_pass"].(string); ok {
			user, err = s.AuthService.ChangePassword(ctx, user, val, newPass)
			if err != nil {
				return nil, err
			}
		}
	}

	err = s.SystemDriver.UpdateSystemUser(p.Context, user, true)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *GraphQLServer) UpsertPluginResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := *cache.Project

	var id string
	if val, ok := p.Args["id"].(string); ok && val != "" {
		id = val
	} else {
		return nil, errors.New("id is required")
	}

	var _type string
	if val, ok := p.Args["type"].(string); ok && val != "" {
		_type = val
	} else {
		return nil, errors.New("plugin type is required")
	}

	var buildCommand map[string]interface{}
	if val, ok := p.Args["plugin"].(map[string]interface{}); ok {
		buildCommand = val
	} else {
		return nil, errors.New("plugin body is needed")
	}

	var _pluginDetails *protobuff.PluginDetails

	switch _type {
	case "local":

		if val, err := plugins.LoadLocalPluginRegistry(s.Cfg, project.Driver); err == nil && val[id] != nil {
			_pluginDetails = val[id]
		} else {
			return nil, errors.New("local plugin is not registered in list")
		}

		var status protobuff.PluginActivateStatus
		if val, ok := buildCommand["activate_status"].(protobuff.PluginActivateStatus); ok {
			status = val
		}
		switch status {
		case protobuff.PluginActivateStatus_activated:
			_pluginDetails.ActivateStatus = protobuff.PluginActivateStatus_deactivated
			project.DefaultStoragePlugin = ""
		case protobuff.PluginActivateStatus_deactivated:
			_pluginDetails.ActivateStatus = 1
			project.DefaultStoragePlugin = id
		}

	case "third_party":
	default:

	}

	err = s.SystemDriver.UpdateProject(p.Context, &project, true)
	if err != nil {
		return nil, err
	}

	return _pluginDetails, nil
}

func (s *GraphQLServer) UpsertFunctionToProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	var functionName string
	if val, ok := p.Args["name"].(string); ok && val != "" {
		functionName = strings.TrimSpace(strcase.ToLowerCamel(val))
	} else {
		return nil, errors.New("function Name Required")
	}

	var isUpdate bool
	if val, ok := p.Args["update"].(bool); ok {
		if val {
			isUpdate = val
		}
	}

	if !isUpdate {
		// check for same func name
		for _, f := range project.Schema.Functions {
			if f.Name == functionName {
				return nil, errors.New("function with this name is already defined")
			}
		}
	}

	// check if function name is valid or not
	if strings.HasPrefix(functionName, "create") || strings.HasPrefix(functionName, "update") || strings.HasPrefix(functionName, "delete") {
		model := functionName[6:len(functionName)]
		for _, m := range cache.Project.Schema.Models {
			if m.Name == strings.ToLower(model) {
				return nil, errors.New(fmt.Sprintf("Function Name `%s` is auto generated by System. Cant use this name", functionName))
			}
		}
	}

	//userID := cache.Param.UserId

	var function *protobuff.CloudFunction

	var oldFunction bool
	// if schema not found then create
	if project.Schema == nil {
		function = &protobuff.CloudFunction{
			Name:      functionName,
			CreatedAt: utility.GetCurrentTime(),
			UpdatedAt: utility.GetCurrentTime(),
		}
		project.Schema = &protobuff.ProjectSchema{
			Functions: []*protobuff.CloudFunction{function},
		}
	} else {
		for _, ct := range project.Schema.Functions {
			if ct.Name == functionName {
				function = ct
				function.UpdatedAt = utility.GetCurrentTime()
				oldFunction = true
				break
			}
		}
		if function == nil {
			function = &protobuff.CloudFunction{
				Name:      functionName,
				CreatedAt: utility.GetCurrentTime(),
				UpdatedAt: utility.GetCurrentTime(),
			}
		}
	}

	if val, ok := p.Args["function_path"].(string); ok && val != "" {
		function.FunctionPath = val
	}

	if val, ok := p.Args["function_provider_id"].(string); ok && val != "" {
		function.FunctionProviderId = val
		var _configuration *protobuff.PluginDetails
		if _val, ok := s.LocalPluginCache[val]; ok {
			_configuration = _val.PluginConfigurations
		}
		function.ProviderExportedVariable = _configuration.ExportedVariable
	}

	if val, ok := p.Args["provider_exported_variable"].(string); ok && val != "" {
		function.ProviderExportedVariable = val
	}

	if val, ok := p.Args["function_exported_variable"].(string); ok && val != "" {
		function.FunctionExportedVariable = val
	}

	//if val, ok := p.Args["type"]
	if val, ok := p.Args["request"].(string); ok {
		function.Request = &protobuff.CloudFunctionRequestResponseType{
			Model: val,
		}
	}

	if val, ok := p.Args["response"].(string); ok {
		function.Response = &protobuff.CloudFunctionRequestResponseType{
			Model: val,
		}
	}

	// update config if found
	if vals, ok := p.Args["env_vars"].([]interface{}); ok {
		if len(vals) > 0 {
			var vars []*protobuff.FunctionEnvVariables
			for _, v := range vals {
				vv := v.(map[string]interface{})
				vars = append(vars, &protobuff.FunctionEnvVariables{
					Key:   vv["key"].(string),
					Value: vv["value"].(string),
				})
			}
			function.EnvVars = vars
		}
	}

	if val, ok := p.Args["runtime_config"].(map[string]interface{}); ok {

		config := function.RuntimeConfig
		for k, v := range val {
			switch k {
			case "runtime":
				config.Runtime = v.(string)
				break
			case "memory":
				config.Memory = int64(v.(int))
				break
			case "handler":
				config.Handler = v.(string)
				break
			case "time_out":
				config.TimeOut = int64(v.(int))
				break
			}
		}
		function.RuntimeConfig = config
	}

	/*switch function.FunctionProviderType {
	case protobuff.FunctionProvider_ViaExtension:
		if plugin, ok := s.LocalPluginCache[function.FunctionProviderName]; ok {
			//if plugin.Lookup("")
			fmt.Println(plugin)
			function.FunctionConnected = true
		}

		if plugin, ok := s.FunctionCache[function.FunctionProviderName]; ok {
			//if plugin.Lookup("")
			fmt.Println(plugin)
			function.FunctionConnected = true
		}

		/*if val, ok := p.Args["remote_function_name"].(string); ok {
			function.FunctionProviderType = protobuff.FunctionProvider_GoPlugin
			if function.ProviderConfig == nil {
				function.ProviderConfig = &protobuff.FunctionProviderConfig{
					RemoteFunctionName: val,
				}
			} else {
				function.ProviderConfig.RemoteFunctionName = val
			}

			if val, ok := p.Args["region"].(string); ok {
				function.ProviderConfig.Region = val
			}

			// fetch all property by func name
			if function.ProviderConfig.Region == "" {
				return nil, errors.New("region is required for remove func assignment")
			}

			functions, err := s.FetchAWSLambdaFunctions(function.ProviderConfig.Region)
			if err != nil {
				return nil, err
			}
			if len(functions) == 0 {
				return nil, errors.New("No function found to connect to")
			}

			var functionToConnect *protobuff.FunctionProviderConfig
			for _, f := range functions {
				if f.RemoteFunctionName == function.ProviderConfig.RemoteFunctionName {
					functionToConnect = f
					break
				}
			}

			if functionToConnect != nil {
				function.ProviderConfig.EnvVars = functionToConnect.EnvVars
				function.ProviderConfig.Configs = functionToConnect.Configs
				function.FunctionConnected = true
			} else {
				return nil, errors.New("Invalid function name given to connect")
			}
		}*/

	/*if function.ProviderConfig == nil {
			return nil, errors.New("Can not set env variable without connecting to a provider")
		}

		// update in aws too
		_, err = s.UpdateAWSLambdaFunctions(function)
		if err != nil {
			return nil, err
		}
		break
	case protobuff.FunctionProvider_GoPlugin:

		// 2. look up a symbol (an exported function or variable)
		// in this case, variable Greeter
		plugin, err := s.PluginLoader(cache.Param.ProjectId, function)
		if err != nil {
			return nil, err
		}

		// 2. look up a symbol (an exported function or variable)
		// in this case, variable Greeter
		pluginLookUp, err := plugin.Lookup(function.ExportedVariable)
		if err != nil {
			return nil, err
		}

		// 3. Assert that loaded symbol is of a desired type
		// in this case interface type Greeter (defined above)
		var loadedPlugin models.Plugins
		loadedPlugin, ok := pluginLookUp.(models.Plugins)
		if !ok {
			return nil, errors.New(fmt.Sprintf(`%s plugin load failed`, function.Name))
		}

		fmt.Println(fmt.Sprintf(`------ Loading %s Function Plugin -------`, function.Name))

		err = loadedPlugin.Init(function.EnvVars)
		if err != nil {
			return nil, err
		}

		function.FunctionConnected = true
	// #todo
	case protobuff.FunctionProvider_NoProvider:
		function.FunctionConnected = false
	}*/

	/*if val, ok := p.Args["function_connected"].(bool); ok {
		if function.FunctionConnected && function.ProviderConfig == nil {
			return nil, errors.New("Try to connect in a proper way")
		}
		function.FunctionConnected = val
	}*/

	if oldFunction {
		err = s.SystemDriver.UpdateProject(p.Context, project, true)
		if err != nil {
			return nil, err
		}
	} else {
		if function.Request == nil || function.Response == nil {
			return nil, errors.New("can not create function without proper Request & Response")
		}
		project.Schema.Functions = append(project.Schema.Functions, function)
		err = s.SystemDriver.UpdateProject(p.Context, project, false)
		if err != nil {
			return nil, err
		}
	}

	return function, nil
}

func (s *GraphQLServer) DeleteFunctionResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	projectId := param.ProjectId
	project, err := s.SystemDriver.GetProject(p.Context, projectId)
	if err != nil {
		return nil, err
	}

	var functionName string
	if val, ok := p.Args["function"].(string); ok && val != "" {
		functionName = val
	} else {
		return nil, errors.New("Function Name is Required")
	}

	// if schema not found then create
	if project.Schema == nil {
		return nil, errors.New("Nothing to Delete")
	} else {
		var found bool
		var index int
		for i, ct := range project.Schema.Functions {
			if ct.Name == functionName {
				found = true
				index = i
				break
			}
		}

		if found {
			project.Schema.Functions = append(project.Schema.Functions[:index], project.Schema.Functions[index+1:]...)
		} else {
			return nil, errors.New("Could not find function to delete")
		}
	}

	err = s.SystemDriver.UpdateProject(p.Context, project, true)
	if err != nil {
		return nil, err
	}

	// #todo delete all the data if so or not

	return project.Schema.Functions, nil
}

func (s *GraphQLServer) CreateConnectionTypeResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
		ctx    = p.Context
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	projectId := param.ProjectId
	project, err := s.SystemDriver.GetProject(ctx, projectId)
	if err != nil {
		return nil, err
	}

	var fromResource string
	if val, ok := p.Args["from"].(string); ok {
		fromResource = strings.TrimSpace(inflection.Singular(strcase.ToLowerCamel(val)))
	} else {
		return nil, errors.New("From Model Needed")
	}

	var toResource string
	if val, ok := p.Args["to"].(string); ok {
		toResource = strings.TrimSpace(inflection.Singular(strcase.ToLowerCamel(val)))
	} else {
		return nil, errors.New("To Model Needed")
	}

	var knownAs string
	if val, ok := p.Args["known_as"].(string); ok {
		knownAs = strings.TrimSpace(inflection.Singular(strcase.ToLowerCamel(val)))
	}

	var connections []*protobuff.ConnectionType

	var fromModelType *protobuff.ModelType
	var toModelType *protobuff.ModelType
	if project.Schema == nil {
		return nil, ae.SchemaIsNil
	} else {
		for _, ct := range project.Schema.Models {
			if ct.Name == fromResource {
				fromModelType = ct
			} else if ct.Name == toResource {
				toModelType = ct
			}
		}

		if fromModelType == nil || toModelType == nil {
			return nil, errors.New("Model Not Found")
		}

		// dont let insert relations without defining any fields
		if len(fromModelType.Fields) == 0 {
			return nil, errors.New(fmt.Sprintf("Can not create relations with %s, because it has no fields.", strings.Title(strings.ToLower(fromModelType.Name))))
		} else if len(toModelType.Fields) == 0 {
			return nil, errors.New(fmt.Sprintf("Can not create relations with %s, because it has no fields.", strings.Title(strings.ToLower(toModelType.Name))))
		}

		// from
		var fromConnectionInfo *protobuff.ConnectionType
		for _, f := range fromModelType.Connections {
			if f.Model == toResource && f.KnownAs == knownAs {
				fromConnectionInfo = f
				break
			}
		}

		if fromConnectionInfo == nil {
			fromConnectionInfo = &protobuff.ConnectionType{
				Model:   toResource,
				Type:    "forward",
				KnownAs: knownAs,
			}
			if val, ok := p.Args["forward_connection_type"]; ok {
				fromConnectionInfo.Relation = val.(string)
			}
			fromModelType.Connections = append(fromModelType.Connections, fromConnectionInfo)
		} else {
			if val, ok := p.Args["forward_connection_type"]; ok {
				fromConnectionInfo.Relation = val.(string)
			}
		}

		connections = append(connections, fromConnectionInfo)

		// to
		var toConnectionInfo *protobuff.ConnectionType
		for _, f := range toModelType.Connections {
			if f.Model == fromResource && f.KnownAs == knownAs {
				toConnectionInfo = f
				break
			}
		}

		if toConnectionInfo == nil {
			toConnectionInfo = &protobuff.ConnectionType{
				Model:   fromResource,
				Type:    "backward",
				KnownAs: knownAs,
			}
			if val, ok := p.Args["reverse_connection_type"]; ok {
				toConnectionInfo.Relation = val.(string)
			}
			toModelType.Connections = append(toModelType.Connections, toConnectionInfo)
		} else {
			if val, ok := p.Args["reverse_connection_type"]; ok {
				toConnectionInfo.Relation = val.(string)
			}
		}

		// used for SQL type driver. For arango it's not implemented or needed
		err = s.GraphQLExecutor.GetProjectDriver(ctx).AddRelationFields(p.Context, fromConnectionInfo, toConnectionInfo)
		if err != nil {
			return nil, err
		}

		err = s.SystemDriver.UpdateProject(p.Context, project, false)
		if err != nil {
			return nil, err
		}

		// for ui purpose
		toConnectionInfo.Model = toResource
		connections = append(connections, toConnectionInfo)
	}

	return connections, nil
}

/*func (s *GraphQLServer) UpdateAWSLambdaFunctions(apitoFunc *protobuff.CloudFunction) (*protobuff.FunctionProviderConfig, error) {

	/*var cred *protobuff.ThirdPartyCredential
	if val, ok := s.PluginConfigurations["aws"]; ok {
		cred = val.Credentials
	} else {
		return nil, errors.New("AWS Credentials are not Set")
	}

	if cred != nil {

		sess, err := session.NewSession(&aws.Config{
			Region:      aws.String(apitoFunc.ProviderConfig.Region),
			Credentials: credentials.NewStaticCredentials(cred.AccessKey, cred.SecretKey, ""),
		})

		svc := lambda.New(sess)
		input := &lambda.UpdateFunctionConfigurationInput{
			FunctionName: aws.String(apitoFunc.ProviderConfig.RemoteFunctionName),
		}

		if apitoFunc.ProviderConfig.Configs != nil {
			input.MemorySize = aws.Int64(apitoFunc.ProviderConfig.Configs.Memory)
			input.Handler = aws.String(apitoFunc.ProviderConfig.Configs.Handler)
			input.Runtime = aws.String(apitoFunc.ProviderConfig.Configs.Runtime)
			input.Timeout = aws.Int64(apitoFunc.ProviderConfig.Configs.TimeOut)
		}

		if apitoFunc.ProviderConfig.EnvVars != nil {
			var envs = make(map[string]*string)
			for _, v := range apitoFunc.ProviderConfig.EnvVars {
				envs[v.Key] = &v.Value
			}
			input.Environment = &lambda.Environment{Variables: envs}
		}

		_, err = svc.UpdateFunctionConfiguration(input)
		if err != nil {
			return nil, err
		}
	}

	return apitoFunc.ProviderConfig, nil
}
*/
