// Package rediskeys centralizes Redis key construction for every GIM service.
package rediskeys

import "strconv"

const (
	authLogoutPrefix   = "gim:auth:logout:"
	presenceUserPrefix = "gim:presence:user:"
)

// AuthLogout returns the key for a SHA-256 token fingerprint. Callers must
// never pass a raw JWT to this helper.
func AuthLogout(tokenFingerprint string) string {
	return authLogoutPrefix + tokenFingerprint
}

// PresenceUser returns the global Presence key for one user. The key contains
// only the stable numeric user ID; per-device IDs and credentials never belong
// in the key namespace.
func PresenceUser(userID uint64) string {
	return presenceUserPrefix + strconv.FormatUint(userID, 10)
}
