package gateio

const (
	PricePercision  = 5
	AmountPercision = 3
)

// from api result
type RawTrade struct {
	TradeId     uint64  `json:"tradeID"`
	OrderNumber uint64  `json:"orderNumber"`
	Pair        string  `json:"pair"`
	Type        string  `json:"type"`
	Rate        string  `json:"rate"`
	Amount      string  `json:"amount"`
	Total       float64 `json:"total"`
	Date        string  `json:"date"`
	TimeUnix    int64   `json:"time_unix"`
	Role        string  `json:"role"`
	Fee         string  `json:"fee"`
	FeeCoin     string  `json:"fee_coin"`
	GtFee       string  `json:"gt_fee"`
	PointFee    string  `json:"point_fee"`
}

type MyTradeHistoryResult struct {
	Result  string     `json:"result"`
	Message string     `json:"message"`
	Code    int        `json:"code"`
	Trades  []RawTrade `json:"trades"`
}

type ResponseBalances struct {
	Result    string `json:"result"`
	Available map[string]string
	Locked    map[string]string
}

type ResponseOrder struct {
	Result       string
	OrderNumber  uint64
	Rate         string
	LeftAmount   string // 剩余数量
	FilledAmount string // 成交数量
	FilledRate   string // 成交价格
	Text         string // 用户自定义订单标识，必须以固定前缀 "t-"开头，不计算前缀的情况下，长度限制为 16 字节，范围 [0-9a-zA-Z-_.]。
	Message      string
}

type ResponseBuy struct {
	Result  string
	Message string
	Code    int

	OrderNumber uint64
	Market      string // 交易对

	CTime   float64 `json:"ctime"` // 时间戳: 1585793595.5014, 秒.毫秒
	Side    int
	Iceberg string

	Rate         string // 下单价格
	FilledRate   string // 成交价格
	LeftAmount   string // 剩余数量
	FilledAmount string // 成交数量

	FeePercentage float64
	FeeValue      string
	FeeCurrency   string
	Fee           string
}

type ResponseSell ResponseOrder

type ResponseCancel struct {
	Result  string
	Message string
}

const (
	OrderStatusOpen      = "open"
	OrderStatusCancelled = "cancelled"
	OrderStatusClosed    = "closed"

	OrderTypeBuy  = "buy"
	OrderTypeSell = "sell"
)

type RawOrderInGetOrder struct {
	OrderNumber  string
	Status       string // 订单状态 open已挂单 cancelled已取消 closed已完成
	CurrencyPair string
	Type         string

	Rate          string // 价格
	Amount        string // 买卖数量
	InitialRate   string // 下单价格
	InitialAmount string // 下单数量
	FilledRate    string
	FilledAmount  string

	FeePercentage float64
	FeeValue      string
	FeeCurrency   string
	Fee           string

	Timestamp int64
}

type ResponseGetOrder struct {
	Result  string
	Message string
	Code    int
	Elapsed string
	Order   RawOrderInGetOrder
}

type RawOrderInOpenOrders struct {
	OrderNumber   string
	Status        string // 记录状态 DONE:完成; CANCEL:取消; REQUEST:请求中
	CurrencyPair  string
	Type          string
	Rate          int64  // 价格
	Amount        string // 买卖数量
	Total         string
	InitialRate   int64  // 下单价格
	InitialAmount string // 下单数量
	FilledRate    string // 成交价格
	FilledAmount  string // 成交数量
	Timestamp     string
}

type ResponseOpenOrders struct {
	Result  string
	Message string
	Code    int64
	Elapsed string
	Orders  []RawOrderInOpenOrders
}

type ResponseTicker struct {
	Result        string // true
	Elapsed       string
	Last          string // 最新成交价
	LowestAsk     string // 卖1，卖方最低价
	HighestBid    string // 买1，买方最高价
	PercentChange string //涨跌百分比
	BaseVolume    string //交易量
	QuoteVolume   string // 兑换货币交易量
	High24hr      string // 24小时最高价
	Low24hr       string // 24小时最低价
}
