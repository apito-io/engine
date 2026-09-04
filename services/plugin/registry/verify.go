package registry

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// EmbeddedApitoCatalogPublicKeyHex is the Apito catalog Ed25519 public key.
// Override with PLUGIN_REGISTRY_PUBLIC_KEY. Private key is a GitHub Actions secret.
const EmbeddedApitoCatalogPublicKeyHex = "ffaacff457ed3c882ad3e947b4e74f646fe8b1f382f21155abde26088e4a7e0b"

// ParsePublicKey decodes a 32-byte Ed25519 public key from hex.
func ParsePublicKey(hexKey string) (ed25519.PublicKey, error) {
	hexKey = strings.TrimSpace(hexKey)
	if hexKey == "" {
		hexKey = EmbeddedApitoCatalogPublicKeyHex
	}
	raw, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, coded(CodeSignatureInvalid, "public key is not valid hex", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, coded(CodeSignatureInvalid, fmt.Sprintf("public key must be %d bytes", ed25519.PublicKeySize), nil)
	}
	return ed25519.PublicKey(raw), nil
}

// VerifyCatalogSignature checks Ed25519 over the exact catalog bytes.
func VerifyCatalogSignature(catalogJSON, sig []byte, pub ed25519.PublicKey) error {
	if len(sig) == 0 {
		return coded(CodeSignatureInvalid, "missing catalog signature", nil)
	}
	// Allow hex-encoded signatures (64-byte binary or 128-char hex).
	if len(sig) == ed25519.SignatureSize*2 || (len(sig) > ed25519.SignatureSize && isHex(sig)) {
		decoded, err := hex.DecodeString(strings.TrimSpace(string(sig)))
		if err != nil {
			return coded(CodeSignatureInvalid, "signature hex decode failed", err)
		}
		sig = decoded
	}
	if len(sig) != ed25519.SignatureSize {
		return coded(CodeSignatureInvalid, "signature length is not 64 bytes", nil)
	}
	if !ed25519.Verify(pub, catalogJSON, sig) {
		return coded(CodeSignatureInvalid, "catalog signature does not match embedded public key", nil)
	}
	return nil
}

func isHex(b []byte) bool {
	for _, c := range strings.TrimSpace(string(b)) {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// DigestSHA256 returns lowercase hex SHA-256.
func DigestSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// SignCatalog is used by tests and the registry generate tool.
func SignCatalog(catalogJSON []byte, priv ed25519.PrivateKey) []byte {
	return ed25519.Sign(priv, catalogJSON)
}

// ParseCatalogJSON decodes and lightly validates schema_version.
func ParseCatalogJSON(b []byte) (*Catalog, error) {
	var cat Catalog
	if err := json.Unmarshal(b, &cat); err != nil {
		return nil, coded(CodeRegistryUnavailable, "catalog JSON is invalid", err)
	}
	if cat.SchemaVersion != 1 {
		return nil, coded(CodeRegistryUnavailable, fmt.Sprintf("unsupported schema_version %d", cat.SchemaVersion), nil)
	}
	return &cat, nil
}
