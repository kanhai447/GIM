// Package rediskeys centralizes Redis key construction for every GIM service.
package rediskeys

import "strconv"

const (
	authLogoutPrefix    = "gim:auth:logout:"
	presenceUserPrefix  = "gim:presence:user:"
	chatDeliveryChannel = "gim:chat:delivery"
)

// AuthLogout returns the key for a SHA-256 token fingerprint. Callers must
// never pass a raw JWT to this helper.
func AuthLogout(tokenFingerprint string) string {
	return authLogoutPrefix + tokenFingerprint
}

// ChatDeliveryChannel is the cluster-wide best-effort private message fanout
// channel. MySQL, not this channel, remains the message source of truth.
func ChatDeliveryChannel() string { return chatDeliveryChannel }

// PresenceUser returns the global Presence key for one user. The key contains
// only the stable numeric user ID; per-device IDs and credentials never belong
// in the key namespace.
func PresenceUser(userID uint64) string {
	return presenceUserPrefix + strconv.FormatUint(userID, 10)
}
