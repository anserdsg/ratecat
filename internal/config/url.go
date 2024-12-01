package config

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net"
	"net/url"
	"strings"
)

type URL struct {
	url.URL
	hash []byte
}

func (u *URL) Decode(val string) error {
	if !strings.Contains(val, "://") {
		sepIdx := strings.Index(val, ":")
		if sepIdx == -1 {
			return fmt.Errorf("invalid url: %s", val)
		}
		host, port, err := net.SplitHostPort(val)
		if err != nil {
			return err
		}
		if len(host) == 0 {
			val = fmt.Sprintf("tcp://127.0.0.1:%s", port)
		} else {
			val = fmt.Sprintf("tcp://%s", net.JoinHostPort(host, port))
		}
	}

	parsed, err := url.Parse(val)
	if err != nil {
		return err
	}
	u.URL = *parsed
	u.hash = nil
	return nil
}

func (u *URL) IsValid() bool {
	return u.Hostname() != "" && u.Port() != ""
}

func (u *URL) UserNameHash() []byte {
	if u.hash != nil {
		return u.hash
	}
	hash := sha256.Sum256([]byte(u.User.Username()))
	u.hash = make([]byte, len(hash))
	copy(u.hash, hash[:])
	return u.hash
}

func (u *URL) NodeID() uint32 {
	hash := u.UserNameHash()
	return binary.LittleEndian.Uint32(hash[:4])
}

func (u *URL) NodeName() string {
	return u.User.Username()
}

func ParseURL(v string) (url URL, err error) {
	err = url.Decode(v)
	return
}
