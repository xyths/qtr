package exchange

type APIKeyPair struct {
	Domain     string `json:"domain"`
	PassPhrase string `json:"passphrase"`
	ApiKey     string `json:"apiKey"`
	SecretKey  string `json:"secretKey"`
}

const (
	CollBalance = "balance"
)

const (
	GET  = "GET"
	POST = "POST"
)
