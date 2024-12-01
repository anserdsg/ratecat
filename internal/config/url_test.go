package config_test

import (
	"testing"

	"github.com/anserdsg/ratecat/v1/internal/config"
	"github.com/stretchr/testify/require"
)

func TestURLDecode(t *testing.T) {
	require := require.New(t)

	var u config.URL
	err := u.Decode("tcp://user:pass@1.2.3.4:1234")
	require.NoError(err)
	require.Equal("tcp", u.Scheme)
	require.Equal("1.2.3.4:1234", u.Host)
	require.Equal("1.2.3.4", u.Hostname())
	require.Equal("1234", u.Port())
	require.Equal("user", u.User.Username())
	password, ok := u.User.Password()
	require.True(ok)
	require.Equal("pass", password)

	err = u.Decode("host:1234")
	require.NoError(err)
	require.Equal("tcp", u.Scheme)
	require.Equal("host", u.Hostname())
	require.Equal("1234", u.Port())

	err = u.Decode("host:invalid")
	require.Error(err)
	err = u.Decode(":invalid")
	require.Error(err)

	err = u.Decode(":1234")
	require.NoError(err)
	require.Equal("127.0.0.1", u.Hostname())
	require.Equal("1234", u.Port())
}

func TestURLHash(t *testing.T) {
	require := require.New(t)

	var u config.URL
	err := u.Decode("tcp://user@1.2.3.4:2345")
	require.NoError(err)
	hash := u.UserNameHash()
	exp := []byte{4, 248, 153, 109, 167, 99, 183, 169, 105, 177, 2, 142, 227, 0, 117, 105, 234, 243, 166, 53, 72, 109, 218, 178, 17, 213, 18, 200, 91, 157, 248, 251}
	require.Equal(exp, hash)

	id := u.NodeID()
	require.Equal(uint32(1838807044), id)
}
