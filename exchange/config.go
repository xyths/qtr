package exchange

type APIKeyPair struct {
	Domain    string `json:"domain"`
	ApiKey    string `json:"apiKey"`
	SecretKey string `json:"secretKey"`
}

const (
	CollBalance = "balance"
)
