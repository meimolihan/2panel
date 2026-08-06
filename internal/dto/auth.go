package dto

type Login struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type LoginInfo struct {
	Token string `json:"token"`
	Name  string `json:"name"`
}

type ChangePassword struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type AuthStatus struct {
	Initialized      bool   `json:"initialized"`
	HasDefaultPasswd bool   `json:"hasDefaultPasswd"`
	DefaultPassword  string `json:"defaultPassword,omitempty"`
	UserName         string `json:"userName"`
}
