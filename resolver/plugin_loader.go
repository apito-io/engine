package resolver

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"plugin"
	"strings"

	"github.com/apito-io/buffers/interfaces"
	plug_buffer "github.com/apito-io/buffers/plugins"
	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/plugins"
	"github.com/apito-io/engine/utility"
	"github.com/tailor-inc/graphql"
)

func (s *GraphQLServer) LoadPlugins() error {

	// Load Local Plugin
	entries, err := os.ReadDir(plugins.LocalPluginDir)
	if err != nil {
		var pathError *fs.PathError
		if errors.As(err, &pathError) {
			if err = os.MkdirAll(plugins.LocalPluginDir, 0770); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	for _, e := range entries {

		s.wg.Add(1)

		go func(_dir os.DirEntry) {
			errs := func() []error {
				defer s.wg.Done()

				var errs []error
				if _, ok := s.LocalPluginCache[_dir.Name()]; !ok {

					var code int32
					dir := fmt.Sprintf(`%s/%s`, plugins.LocalPluginDir, _dir.Name())

					code, err = s.CheckPluginExists(dir)
					if err != nil {
						errs = append(errs, err)
					}

					if code == 0 {
						fmt.Println(fmt.Sprintf("Local Plugin %s Not Found", _dir.Name()))

						// build first
						/*s.PublishSystemMessage(param.UserId, &models.SubscriptionEvent{
							Type:    "info",
							Message: fmt.Sprintf("Building Local Plugin : %s", _dir.Name()),
						})
						err = s.BuildPlugin(dir)
						if err != nil {
							errs = append(errs, err)
							goto skipPublish
						}*/
						goto skipLoad
					}

					fmt.Println(fmt.Sprintf("Loading Local Plugin %s", _dir.Name()))

					// for the first time plugin loader. there is no project driver
					_, err = s.LoadLocalPlugin(context.Background(), dir, nil)
					if err != nil {
						errs = append(errs, err)
					}
				}
			skipLoad:
				return errs
			}()

			if len(errs) > 0 {
				for _, err := range errs {
					fmt.Println(fmt.Sprintf("Error Loading Local Plugin %s", err.Error()))
				}
				//return nil, errors.New("build error found in local plugin")
			}
		}(e)
	}

	//var thirdPartyLoadedPlugin []*models.LoadedPlugin

	// load installed plugin
	/*for _, _plugin := range _project.Plugins {
		if _plugin.Enable {

			id := _plugin.Id

			_dir := "extensions"

			// load module
			// 1. open the so file to load the symbols
			modulePath := fmt.Sprintf(`plugins/%s/%s/%s/%s.so`, _project.Id, _dir, id, id)
			if _, err := os.Stat(modulePath); os.IsNotExist(err) {
				// Do whatever is required on module not existing
				return nil, errors.New(fmt.Sprintf("enabled plugin load error [%s]. please check", modulePath))
			}

			plug, err := plugin.Open(modulePath)
			if err != nil {
				fmt.Println(err)
				continue // skip the loading
			}
			s.LocalPluginCache[id] = &models.LocalPluginCache{
				Plugin:         plug,
				Configurations: _plugin,
			}

			// 2. look up a symbol (an exported function or variable)
			// in this case, variable Greeter
			pluginLookUp, err := s.LocalPluginCache[id].Plugin.Lookup(_plugin.ExportedVariable)
			if err != nil {
				fmt.Println(err)
			}

			// 3. Assert that loaded symbol is of a desired type
			// in this case interface type Greeter (defined above)
			var loadedPlugin models.Plugins
			loadedPlugin, ok := pluginLookUp.(models.Plugins)
			if !ok {
				fmt.Println(fmt.Sprintf("%s plugin load failed", id))
			}

			fmt.Println(fmt.Sprintf(`------ Loading %s Plugin -------`, id))

			err = loadedPlugin.Init(_plugin.EnvVars)
			if err != nil {
				fmt.Println(fmt.Sprintf("%s %s load failed", id, err.Error()))
			}

			_type, err := loadedPlugin.ExtensionType()
			if err != nil {
				fmt.Println(fmt.Sprintf("%s %s load failed", id, err.Error()))
			}

			err = loadedPlugin.Migration()
			if err != nil {
				fmt.Println(fmt.Sprintf("%s %s load failed", id, err.Error()))
			}

			// 4. use the module
			graphqlSchemas, err := loadedPlugin.SchemaRegister()
			if err != nil {
				fmt.Println(fmt.Sprintf("%s %s load failed", id, err.Error()))
			}

			for k, v := range graphqlSchemas.Queries {
				if val := extensionLoader.RawSchema.Queries[k]; val == nil {
					extensionLoader.RawSchema.Queries[k] = v
				} else {
					fmt.Println(fmt.Sprintf(`the query '%s' on '%s' already found on another _plugin. please change the id. ignoring this query`, k, id))
				}
			}

			for k, v := range graphqlSchemas.Mutations {
				if val := extensionLoader.RawSchema.Mutations[k]; val == nil {
					extensionLoader.RawSchema.Mutations[k] = v
				} else {
					fmt.Println(fmt.Sprintf(`the mutation '%s' on '%s' already found on another _plugin. please change the id. ignoring this query`, k, id))
				}
			}

			// 4. use the module
			// #todo check for duplicate routes before registering
			routes, err := loadedPlugin.RESTApiRegister()
			if err != nil {
				fmt.Println(err.Error())
			}

			extensionLoader.Routes = append(extensionLoader.Routes, routes...)

			switch _type {
			case protobuff.PluginType_Extension:
			case protobuff.PluginType_Function:
				// if there is no function then avoid loading logic related plugins
				if _project.Schema != nil && _project.Schema.Functions == nil {
					continue
				}

				// search for function that is connected to extension
				var _cloudFunction *protobuff.CloudFunction
				for _, cf := range _project.Schema.Functions {
					if id == cf.FunctionProviderName {
						_cloudFunction = cf
						break
					}
				}

				// transfer the exported variable
				_cloudFunction.ExportedVariable = _plugin.ExportedVariable
			case protobuff.PluginType_Media:
			default:
			}

			/*_, err = _plugin.Execute()
			if err != nil {
				fmt.Println(fmt.Sprintf("%s %s load failed", id, err.Error()))
			}
		}
	}*/

	// update the project
	/*err = s.SystemDriver.UpdateProject(_project, true)
	if err != nil {
		return err
	}*/

	return nil
}

func (s *GraphQLServer) CheckPluginExists(dir string) (int32, error) {
	file := fmt.Sprintf(`%s/%s`, dir, "main.so")
	if _, err := os.Stat(file); err == nil {
		return 1, nil
	} else {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 2, err
	}
}

func (s *GraphQLServer) LoadLocalPlugin(ctx context.Context, _dir string, driver *protobuff.DriverCredentials) (*protobuff.PluginDetails, error) {

	_file := fmt.Sprintf(`%s/%s`, _dir, "main.so")

	if _, err := os.Stat(_file); os.IsNotExist(err) {
		// Do whatever is required on module not existing
		return nil, errors.New(fmt.Sprintf("local plugin load error [%s]. please check", _file))
	}

	plug, err := plugin.Open(_file)
	if err != nil {
		return nil, err
	}

	_tmpIds := strings.Split(_dir, "/")
	_tmpId := _tmpIds[len(_tmpIds)-1]

	var _pluginDetails *protobuff.PluginDetails
	if val, err := plugins.LoadLocalPluginRegistry(s.Cfg, driver); err == nil && val[_tmpId] != nil {
		_pluginDetails = val[_tmpId]
	} else {
		return nil, errors.New("local plugin is not registered in list")
	}

	// 2. look up a symbol (an exported function or variable)
	// in this case, variable Greeter
	pluginLookUp, err := plug.Lookup(_pluginDetails.ExportedVariable)
	if err != nil {
		return nil, err
	}

	pluginId := _pluginDetails.Id

	switch _pluginDetails.Type {
	case protobuff.PluginType_NormalPlugin:

		// 1. Identify and load the plugin
		var loadedPlugin interfaces.NormalPluginInterface
		loadedPlugin, ok := pluginLookUp.(interfaces.NormalPluginInterface)
		if !ok {
			return nil, errors.New(fmt.Sprintf("%s plugin load failed", pluginId))
		}

		var envs []*plug_buffer.EnvVariables
		for _, v := range _pluginDetails.EnvVars {
			envs = append(envs, &plug_buffer.EnvVariables{Key: v.Key, Value: v.Value})
		}

		// 2. init the plugin
		err = loadedPlugin.Init(envs)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("%s %s Init Call failed", pluginId, err.Error()))
		}

		// 3. run the migration
		err = loadedPlugin.Migration()
		if err != nil {
			fmt.Println(fmt.Sprintf("%s %s load failed", pluginId, err.Error()))
		}

		// 4. register schema if any
		graphqlSchemas, err := loadedPlugin.SchemaRegister()
		if err != nil {
			fmt.Println(fmt.Sprintf("%s %s load failed", pluginId, err.Error()))
		}

		if graphqlSchemas != nil {
			localSchema := plug_buffer.ThirdPartyGraphQLSchemas{}
			for k, v := range graphqlSchemas.Queries {
				if val := localSchema.Queries[k]; val == nil {
					localSchema.Queries[k] = v
				} else {
					fmt.Println(fmt.Sprintf(`the query '%s' on '%s' already found on another _plugin. please change the id. ignoring this query`, k, pluginId))
				}
			}
			for k, v := range graphqlSchemas.Mutations {
				if val := localSchema.Mutations[k]; val == nil {
					localSchema.Mutations[k] = v
				} else {
					fmt.Println(fmt.Sprintf(`the mutation '%s' on '%s' already found on another _plugin. please change the id. ignoring this query`, k, pluginId))
				}
			}
			s.LocalPluginGraphQLSchemas <- &localSchema
		}

		// 4. use the module
		// #todo check for duplicate routes before registering
		routes, err := loadedPlugin.RESTApiRegister()
		if err != nil {
			fmt.Println(err.Error())
		}

		if routes != nil {
			var localRoutes []*plug_buffer.ThirdPartyRESTApi
			localRoutes = append(localRoutes, routes...)
			s.LocalPluginRoutes <- localRoutes
		}
	case protobuff.PluginType_Function:

		if !utility.ArrayContains(s.FunctionProviderIds, pluginId) {
			s.FunctionProviderIds = append(s.FunctionProviderIds, pluginId)
		}

		//var _localPluginExecutableFunctions *protobuff.CloudFunction

		// if there is no function then avoid loading logic related plugins
		/*if schema != nil && schema.Functions != nil {
			// search for function that is connected to extension
			for _, cf := range schema.Functions {
				if _pluginDetails.Id == cf.FunctionProviderId {
					cf.ProviderExportedVariable = _pluginDetails.ExportedVariable

					// check if connected then load it
					if cf.FunctionConnected && cf.

					_localPluginExecutableFunctions = cf
					break
				}
			}
		}*/

		/*// transfer the exported variable
		s.Lock()
		_pluginDetails.LoadStatus = protobuff.PluginLoadStatus_Loaded
		s.FunctionCache[pluginId] = &models.FunctionCache{
			Functions:         plug,
			FuncConfiguration: _localPluginExecutableFunctions,
		}
		s.Unlock()*/
	case protobuff.PluginType_Storage:

		// 1. Identify and load the plugin
		var loadedPlugin interfaces.StoragePluginInterface
		loadedPlugin, ok := pluginLookUp.(interfaces.StoragePluginInterface)
		if !ok {
			return nil, errors.New(fmt.Sprintf("%s plugin load failed", pluginId))
		}

		// 2. Build env based on plugin details env
		var envs []*plug_buffer.EnvVariables
		for _, v := range _pluginDetails.EnvVars {
			envs = append(envs, &plug_buffer.EnvVariables{Key: v.Key, Value: v.Value})
		}

		// 3. init the plugin
		err = loadedPlugin.Init(envs)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("%s %s Init Call failed", pluginId, err.Error()))
		}

		// 4. run the migration
		err = loadedPlugin.Migration(ctx)
		if err != nil {
			fmt.Println(fmt.Sprintf("%s %s load failed", pluginId, err.Error()))
		}

		// 5. register schema if any
		graphqlSchemas, err := loadedPlugin.SchemaRegister(ctx)
		if err != nil {
			fmt.Println(fmt.Sprintf("%s %s load failed", pluginId, err.Error()))
		}

		if graphqlSchemas != nil {
			localSchema := plug_buffer.ThirdPartyGraphQLSchemas{
				Queries:   graphql.Fields{},
				Mutations: graphql.Fields{},
			}
			for k, v := range graphqlSchemas.Queries {
				if val := localSchema.Queries[k]; val == nil {
					localSchema.Queries[k] = v
				} else {
					fmt.Println(fmt.Sprintf(`the query '%s' on '%s' already found on another _plugin. please change the id. ignoring this query`, k, pluginId))
				}
			}
			for k, v := range graphqlSchemas.Mutations {
				if val := localSchema.Mutations[k]; val == nil {
					localSchema.Mutations[k] = v
				} else {
					fmt.Println(fmt.Sprintf(`the mutation '%s' on '%s' already found on another _plugin. please change the id. ignoring this query`, k, pluginId))
				}
			}
			s.LocalPluginGraphQLSchemas <- &localSchema
			fmt.Println("successfully loaded graphql schema")
		}

		// 5. use the module
		// #todo check for duplicate routes before registering
		routes, err := loadedPlugin.RESTApiRegister(ctx)
		if err != nil {
			fmt.Println(err.Error())
		}

		if routes != nil {
			var localRoutes []*plug_buffer.ThirdPartyRESTApi
			localRoutes = append(localRoutes, routes...)
			s.LocalPluginRoutes <- localRoutes
		}

		if !utility.ArrayContains(s.StorageProviderIds, pluginId) {
			s.StorageProviderIds = append(s.StorageProviderIds, pluginId)
		}
	default:
	}

	if !utility.ArrayContains(s.InstalledPluginList, pluginId) {
		s.InstalledPluginList = append(s.InstalledPluginList, pluginId)
	}

	/*_, err = _plugin.Execute()
	if err != nil {
		fmt.Println(fmt.Sprintf("%s %s load failed", id, err.Error()))
	}
	*/

	s.Lock()
	_pluginDetails.LoadStatus = protobuff.PluginLoadStatus_Loaded
	s.LocalPluginCache[pluginId] = &models.PluginCache{
		Plugin:               plug,
		PluginConfigurations: _pluginDetails,
	}
	s.Unlock()

	return _pluginDetails, nil
}

func (s *GraphQLServer) BuildPlugin(_dir string) error {
	fmt.Println(fmt.Sprintf(`------ Building Local Plugin '%s' -------`, _dir))
	_file := fmt.Sprintf(`%s/%s`, _dir, "main.go")

	args := []string{"build", "-buildmode=plugin"}
	if s.Cfg.Environment == "develop" || s.Cfg.Environment == "local" {
		args = append(args, "-gcflags", "all=-N -l")
	}
	args = append(args, []string{"-o", _dir, _file}...)

	cmd := exec.Command("go", args...)
	cmd.Env = os.Environ()
	//cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	//cmd.Env = append(cmd.Env, "GOOS=linux")
	fmt.Println("building plugin with arg : " + strings.Join(cmd.Args, " "))
	output, err := cmd.CombinedOutput()
	fmt.Println("building done |")
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + string(output))
	}
	return err
}

func (s *GraphQLServer) PluginLoader(projectId string, _conf *protobuff.CloudFunction) (*plugin.Plugin, error) {

	_dir := "functions"
	_path := fmt.Sprintf(`plugins/%s/%s/%s`, projectId, _dir, _conf.FunctionPath)

	if _, err := os.Stat(_path); os.IsNotExist(err) {
		// Do whatever is required on module not existing
		return nil, errors.New(fmt.Sprintf("plugin file not found [%s]. please check", _path))
	}

	plug, err := plugin.Open(_path)
	if err != nil {
		return nil, err
	}

	var _plugin *plugin.Plugin

	if p, ok := s.FunctionCache[_conf.Name]; ok {
		_plugin = p.Functions
	} else {
		s.FunctionCache[_conf.Name] = &models.FunctionCache{
			Functions: plug,
			//FuncConfiguration: _conf,
		}
		_plugin = plug
	}

	return _plugin, nil
}
