package auth

// AuthConfig holds auth service specific configuration
type AuthConfig struct {
	JWTSecret     string
	JWTExpireHour int
}

// NewAuthConfig creates auth config from main config
func NewAuthConfig(secret string, expireHour int) *AuthConfig {
	return &AuthConfig{
		JWTSecret:     secret,
		JWTExpireHour: expireHour,
	}
}
