package api

import (
	"net/http"
	"strings"
)

var socialEndpointPrefixes = map[string][]string{
	http.MethodGet: {
		"/ob/status",
		"/ob/peers",
		"/ob/config",
		"/ob/settings",
		"/ob/closestpeers",
		"/ob/followers",
		"/ob/following",
		"/ob/profile",
		"/ob/followsme",
		"/ob/isfollowing",
		"/ob/chatmessages",
		"/ob/chatconversations",
		"/ob/notifications",
		"/ob/image",
		"/ob/avatar",
		"/ob/header",
		"/ob/healthcheck",
		"/ob/ipns",
		"/ob/resolveipns",
		"/ob/peerinfo",
		"/ob/posts",
		"/ob/post",
		"/ob/scanofflinemessages",
	},
	http.MethodPost: {
		"/ob/follow",
		"/ob/unfollow",
		"/ob/profile",
		"/ob/images",
		"/ob/settings",
		"/ob/avatar",
		"/ob/header",
		"/ob/chat",
		"/ob/signmessage",
		"/ob/verifymessage",
		"/ob/groupchat",
		"/ob/markchatasread",
		"/ob/marknotificationasread",
		"/ob/marknotificationsasread",
		"/ob/fetchprofiles",
		"/ob/blocknode",
		"/ob/shutdown",
		"/ob/publish",
		"/ob/purgecache",
		"/ob/post",
		"/ob/hashmessage",
	},
	http.MethodPut: {
		"/ob/profile",
		"/ob/settings",
		"/ob/post",
	},
	http.MethodPatch: {
		"/ob/settings",
		"/ob/profile",
	},
	http.MethodDelete: {
		"/ob/chatmessage",
		"/ob/chatconversation",
		"/ob/notifications",
		"/ob/blocknode",
		"/ob/post",
	},
}

// socialOnlyAllowedEndpoint is the HTTP composition boundary for a social
// node. Wallet routes remain available for optional tipping and payments.
func socialOnlyAllowedEndpoint(path, method string) bool {
	if method == http.MethodOptions {
		return strings.HasPrefix(path, "/ob/") || strings.HasPrefix(path, "/wallet/")
	}
	if method == http.MethodHead {
		method = http.MethodGet
	}
	if strings.HasPrefix(path, "/wallet/") {
		return true
	}
	for _, prefix := range socialEndpointPrefixes[method] {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
