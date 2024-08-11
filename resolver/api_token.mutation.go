package resolver

import (
	"errors"
	"time"

	"github.com/apito-io/buffers/protobuff"
	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/services"
	"github.com/apito-io/engine/utility"
	"github.com/labstack/echo/v4"
	"github.com/tailor-inc/graphql"
)

func (s *GraphQLServer) GenerateApiTokenResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	projectId := param.ProjectId

	var name string
	if val, ok := p.Args["name"].(string); ok {
		name = val
	} else {
		return nil, errors.New("name Id Required")
	}

	var duration string
	if val, ok := p.Args["duration"].(string); ok {
		duration = val
	} else {
		return nil, errors.New("Duration is Required")
	}

	project, err := s.SystemDriver.GetProject(p.Context, projectId)
	if err != nil {
		return nil, err
	}

	t := services.GetBrankaToken(s.Cfg, s.SystemDriver)

	parseDuration, _ := time.Parse(time.RFC3339, duration)
	current := utility.GetCurrentTimeObject()
	ttl := parseDuration.Sub(current).Seconds()

	if ttl <= 0 {
		return nil, errors.New("Invalid Date Format or Backward Date is given")
	}

	ttlFinal := uint32(uint64(ttl))

	token, err := t.GenerateProjectToken(param.UserId, project.Id, ttlFinal)
	if err != nil {
		return nil, err
	}

	project.Tokens = append(project.Tokens, &protobuff.APIToken{
		Name:   name,
		Token:  *token,
		Expire: duration,
	})

	err = s.SystemDriver.UpdateProject(p.Context, project, false)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"token": *token,
	}, nil
}

func (s *GraphQLServer) DeleteApiTokenResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	projectId := param.ProjectId

	var token string
	if val, ok := p.Args["token"].(string); ok {
		token = val
	} else {
		return nil, ae.TokenIsRequired
	}

	var duration string
	if val, ok := p.Args["duration"].(string); ok {
		duration = val
	} else {
		return nil, errors.New("Duration is Required")
	}

	ext, err := s.BlankaTokenService.Validate(p.Context, token)
	if err != nil {
		return nil, err
	}

	project, err := s.SystemDriver.GetProject(p.Context, projectId)
	if err != nil {
		return nil, err
	}

	for i, t := range project.Tokens {
		if t.Token == token {
			project.Tokens = append(project.Tokens[:i], project.Tokens[i+1:]...)
		}
	}

	err = s.SystemDriver.UpdateProject(p.Context, project, true)
	if err != nil {
		return nil, err
	}

	parseDuration, _ := time.Parse(time.RFC3339, duration)
	alreadyExpired := parseDuration.Sub(time.Now()).Hours()
	if alreadyExpired > 0.0 { // expire the token
		expiredToken := map[string]interface{}{
			"id":        ext.TokenUniqueId,
			"_key":      ext.TokenUniqueId,
			"expire_at": duration,
		}

		err = s.SystemDriver.BlacklistAToken(p.Context, expiredToken)
		if err != nil {
			return nil, err
		}
	}

	return map[string]interface{}{
		"msg": "Token Deleted",
	}, nil
}
