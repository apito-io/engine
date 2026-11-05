package plugin

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// RedactPluginConfigSecrets blanks env values whose key contains PASSWORD, SECRET, HASH, TOKEN, or PRIVATE.
func RedactPluginConfigSecrets(raw []byte) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	redactSecretNode(&doc)
	return yaml.Marshal(&doc)
}

func redactSecretNode(node *yaml.Node) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range node.Content {
			redactSecretNode(child)
		}
	case yaml.MappingNode:
		var keyValue, secretValue *yaml.Node
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			val := node.Content[i+1]
			if key.Kind == yaml.ScalarNode && secretEnvKey(key.Value) && val.Kind == yaml.ScalarNode {
				val.Value = ""
				val.Tag = "!!str"
			}
			if key.Kind == yaml.ScalarNode && key.Value == "key" && val.Kind == yaml.ScalarNode {
				keyValue = val
			}
			if key.Kind == yaml.ScalarNode && key.Value == "value" && val.Kind == yaml.ScalarNode {
				secretValue = val
			}
			redactSecretNode(val)
		}
		if keyValue != nil && secretValue != nil && secretEnvKey(keyValue.Value) {
			secretValue.Value = ""
			secretValue.Tag = "!!str"
		}
	}
}

func secretEnvKey(key string) bool {
	upper := strings.ToUpper(key)
	for _, part := range []string{"PASSWORD", "SECRET", "HASH", "TOKEN", "PRIVATE"} {
		if strings.Contains(upper, part) {
			return true
		}
	}
	return false
}
