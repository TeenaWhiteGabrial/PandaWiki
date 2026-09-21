package config

import (
	"os"
	"strconv"
	"sync/atomic"

	"github.com/spf13/viper"
)

// PermissionLevelsDisabled suspends permission-level checks without changing stored rules.
// Authentication and API-token scope checks must remain enforced.
type permissionResolver struct{ disabled func() bool }

var runtimePermissionResolver atomic.Pointer[permissionResolver]

// BindPermissionsResolver makes the persisted administrator setting authoritative.
func BindPermissionsResolver(disabled func() bool) {
	runtimePermissionResolver.Store(&permissionResolver{disabled: disabled})
}

func PermissionLevelsDisabled() bool {
	if resolver := runtimePermissionResolver.Load(); resolver != nil {
		return resolver.disabled()
	}
	return DefaultPermissionLevelsDisabled()
}

// DefaultPermissionLevelsDisabled is only used to initialize a missing database setting.
func DefaultPermissionLevelsDisabled() bool {
	if value := os.Getenv("DISABLE_PERMISSION_LEVELS"); value != "" {
		disabled, _ := strconv.ParseBool(value)
		return disabled
	}
	return viper.GetBool("permissions.levels_disabled")
}
