package controller

import (
	"errors"
	"strings"
)

const (
	accessTokenTypeCLI = "cli"
	accessTokenTypeSDK = "sdk"
	accessTokenTypeMCP = "mcp"
)

// normalizeAccessTokenType maps request token_type to cli|sdk|mcp. Empty defaults to cli.
func normalizeAccessTokenType(tokenType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(tokenType)) {
	case "", accessTokenTypeCLI:
		return accessTokenTypeCLI, nil
	case accessTokenTypeSDK:
		return accessTokenTypeSDK, nil
	case accessTokenTypeMCP:
		return accessTokenTypeMCP, nil
	default:
		return "", errors.New("invalid token_type: must be cli, sdk, or mcp")
	}
}

func accessTokenInnerType(accessType string) string {
	return accessType + "_token"
}

func prefixAccessToken(accessType, raw string) string {
	return accessType + "-" + raw
}

// syncTokensMatch returns true when stored and requested refer to the same access token.
func syncTokensMatch(stored, requested string) bool {
	if stored == "" || requested == "" {
		return false
	}
	if stored == requested {
		return true
	}
	for _, p := range []string{"cli-", "sdk-", "mcp-"} {
		if strings.HasPrefix(stored, p) && stored[4:] == requested {
			return true
		}
		if strings.HasPrefix(requested, p) && requested[4:] == stored {
			return true
		}
		if strings.HasPrefix(stored, p) && strings.HasPrefix(requested, p) && stored == requested {
			return true
		}
	}
	return false
}
