package config

import (
	"github.com/spf13/viper"
	"strings"
	"testing"
)

func TestPermissionsConfigAndEnvironmentOverride(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.SetConfigType("yml")
	if err := viper.ReadConfig(strings.NewReader("permissions:\n  levels_disabled: true\n")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DISABLE_PERMISSION_LEVELS", "")
	if !PermissionLevelsDisabled() {
		t.Fatal("configuration was not loaded")
	}
	t.Setenv("DISABLE_PERMISSION_LEVELS", "false")
	if PermissionLevelsDisabled() {
		t.Fatal("environment override was ignored")
	}
	t.Setenv("DISABLE_PERMISSION_LEVELS", "")
	if err := viper.ReadConfig(strings.NewReader("permissions:\n  levels_disabled: false\n")); err != nil {
		t.Fatal(err)
	}
	if PermissionLevelsDisabled() {
		t.Fatal("configuration did not restore permissions")
	}
}

func TestPermissionLevelsDisabledIsExplicit(t *testing.T) {
	for _, value := range []string{"", "false", "invalid", "true"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("DISABLE_PERMISSION_LEVELS", value)
			if PermissionLevelsDisabled() != (value == "true") {
				t.Fatal("unexpected permission mode")
			}
		})
	}
}
