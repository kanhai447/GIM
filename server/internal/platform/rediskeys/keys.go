// Package rediskeys centralizes Redis key construction for every GIM service.
package rediskeys

const authLogoutPrefix = "gim:auth:logout:"

// AuthLogout returns the key for a SHA-256 token fingerprint. Callers must
// never pass a raw JWT to this helper.
func AuthLogout(tokenFingerprint string) string {
	return authLogoutPrefix + tokenFingerprint
}
