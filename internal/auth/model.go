package auth

type GoogleLoginRequest struct {
	Credential string `json:"credential"`
}
type LoginResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresIn   int64        `json:"expires_in"`
	User        UserResponse `json:"user"`
}
type UserResponse struct {
	ID       string `json:"id"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}
type VerifiedIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
}
