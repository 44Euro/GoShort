package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"goshort/internal/config"
)

func TestTheAdminRoleIsOnUnlessItIsTurnedOff(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/goshort")
	t.Setenv("JWT_SECRET", "secret")

	c, err := config.Load()
	require.NoError(t, err)
	require.True(t, c.AdminEnabled, "one instance doing both jobs is the ordinary deployment")

	for _, off := range []string{"0", "false", "FALSE"} {
		t.Setenv("ADMIN_ENABLED", off)
		c, err := config.Load()
		require.NoError(t, err)
		require.False(t, c.AdminEnabled, "ADMIN_ENABLED=%s should drop the admin role", off)
	}

	// ค่าที่ parse ไม่ออกต้องตกกลับไปหาปริยาย ไม่ใช่กลายเป็นปิดเงียบ ๆ
	t.Setenv("ADMIN_ENABLED", "yes-please")
	c, err = config.Load()
	require.NoError(t, err)
	require.True(t, c.AdminEnabled)
}
