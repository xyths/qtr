package exchange

type APIKeyPair struct {
	ApiKey    string `json:"apiKey"`
	SecretKey string `json:"secretKey"`
}

const (
	CollBalance = "balance"
)
