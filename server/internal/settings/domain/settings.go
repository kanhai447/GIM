package domain

type Site struct {
	Name                string `json:"name"`
	Version             string `json:"version"`
	Notice              string `json:"notice"`
	RegistrationEnabled bool   `json:"registrationEnabled"`
}

type OpenLogin struct {
	Enabled bool `json:"enabled"`
}

// Info contains only public business settings. Infrastructure endpoints,
// signing material, credentials, and third-party secrets are never settings.
type Info struct {
	Site      Site      `json:"site"`
	OpenLogin OpenLogin `json:"openLogin"`
}

func Defaults() Info {
	return Info{Site: Site{Name: "GIM", Version: "v1", RegistrationEnabled: true}}
}
