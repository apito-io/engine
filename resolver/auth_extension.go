package resolver

/*func (s *GraphQLServer) ApitoLogInResolverFn(p graphql.ResolveParams) (interface{}, error) {
	var email string
	if val, ok := p.Args["email"].(string); ok {
		email = val
	}

	var phone string
	if val, ok := p.Args["phone"].(string); ok {
		phone = val
	}

	var secret string
	if val, ok := p.Args["secret"].(string); ok {
		secret = val
	} else {
		return nil, errors.New("Secret is needed")
	}

	return s.loginHndlr(phone, email, secret)
}

func (s *GraphQLServer) loginHndlr(phone, email, secret string) (interface{}, error) {

	user, err := s.GetProjectDriver().GetProjectUser(phone, email, s.Param.ProjectId)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, errors.New("user Not Found")
	}

	// check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Data["secret"].(string)), []byte(secret)); err != nil {
		return nil, errors.New("invalid Authentication Request")
	}

	var authExtension *protobuff.ExtensionDetails
	if val, ok := s.PluginConfigurations["auth"]; ok {
		authExtension = val
	} else {
		return nil, errors.New("auth extension not found")
	}

	param := s.Param
	param.Role = &protobuff.Role{
		Id: authExtension.Role,
	}
	param.UserId = user.Id
	param.Email = email

	token, err := s.JwtService.GenerateIdToken(param)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.JwtService.GenerateRefreshToken(param, 1)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id_token":      token,
		"refresh_token": refreshToken,
	}, nil
}

func (s *GraphQLServer) HandleAuth(_case string, req map[string]interface{}) (interface{}, error) {
	var email string
	if val, ok := req["email"].(string); ok {
		email = val
	}

	var phone string
	if val, ok := req["phone"].(string); ok {
		phone = val
	}

	var secret string
	if val, ok := req["secret"].(string); ok {
		secret = val
	} else {
		return nil, errors.New("Secret is needed")
	}

	switch _case {
	case "login":
		return s.loginHndlr(phone, email, secret)
	case "register":
		return s.registerHndlr(phone, email, secret)
	}
	return nil, errors.New("Request not Understood")
}

func (s *GraphQLServer) ApitoLoggedInUserResolverFn(p graphql.ResolveParams) (interface{}, error) {

	if !s.Param.Role.IsProjectUser {
		return nil, errors.New("this method only works on user based JWT token. Can not preview on Admin")
	}

	user, err := s.GetProjectDriver().GetLoggedInProjectUser(s.Param)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, errors.New("User Not Found")
	}

	resp := map[string]interface{}{
		"id":         user.Id,
		"first_name": user.Data["first_name"],
		"email":      user.Data["email"],
		"phone":      user.Data["phone"],
	}
	if val, ok := user.Data["avatar"].(map[string]interface{}); ok {
		resp["avatar"] = val["url"]
	}
	return resp, nil
}

func (s *GraphQLServer) registerHndlr(phone, email, secret string) (interface{}, error) {
	user, err := s.GetProjectDriver().GetProjectUser(phone, email, s.Param.ProjectId)
	if err != nil {
		return nil, err
	}

	if user != nil {
		return nil, errors.New("User is already registered")
	}

	// GenerateIdToken "hash" to store from user password
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	newPass := string(hash)

	data := map[string]interface{}{
		"secret": newPass,
	}

	if phone != "" {
		data["phone"] = phone
	}
	if email != "" {
		data["email"] = email
	}

	id := utility.NewID()
	user = &shared.DefaultDocumentStructure{
		Key:  id,
		Id:   id,
		Type: "user",
		Data: data,
		//Role: s.AddOns.Auth.AuthUserRoles,
		Meta: &protobuff.MetaField{
			CreatedAt: utility.GetCurrentTime(),
			UpdatedAt: utility.GetCurrentTime(),
			CreatedBy: &protobuff.UserMeta{
				Id: s.Param.UserId,
			},
			LastModifiedBy: &protobuff.UserMeta{
				Id: s.Param.UserId,
			},
			Status: "draft",
		},
	}

	// insert the user
	_, err = s.GetProjectDriver().AddDocumentToProject(s.Param.ProjectId, "user", user)
	if err != nil {
		return nil, err
	}

	var authExtension *protobuff.ExtensionDetails
	if val, ok := s.PluginConfigurations["auth"]; ok {
		authExtension = val
	} else {
		return nil, errors.New("auth extension not found")
	}

	param := s.Param
	param.Role = &protobuff.Role{Id: authExtension.Role}
	param.UserId = id

	token, err := s.JwtService.GenerateIdToken(param)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.JwtService.GenerateRefreshToken(param, 1)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":            user.Id,
		"id_token":      token,
		"refresh_token": refreshToken,
	}, nil
}

func (s *GraphQLServer) ApitoRegisterResolverFn(p graphql.ResolveParams) (interface{}, error) {
	var phone string
	if val, ok := p.Args["phone"].(string); ok {
		phone = val
	}

	var email string
	if val, ok := p.Args["email"].(string); ok {
		email = val
	}

	var secret string
	if val, ok := p.Args["secret"].(string); ok {
		secret = val
	} else {
		return nil, errors.New("Secret is needed")
	}
	return s.registerHndlr(phone, email, secret)
}
*/
