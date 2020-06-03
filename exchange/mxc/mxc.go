package mxc

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

type MXC struct {
	Domain string

	Key    string
	Secret string
}

func NewMXC(domain, key, secret string) *MXC {
	return &MXC{
		Domain: domain,
		Key:    key,
		Secret: secret,
	}
}

func (mxc *MXC) Timestamp() (timestampMs uint64, err error) {
	url := mxc.Domain + "/open/api/v2/common/timestamp"
	params := make(map[string]string)
	var resp ResponseTimestamp
	if err := mxc.requestGet(url, params, &resp); err != nil {
		return 0, err
	} else {
		//log.Printf("timestamp = %d", resp.Data)
		return resp.Data, nil
	}
}

func (mxc *MXC) OpenOrders(symbol, limit, start string) ([]RawOrder, error) {
	url := mxc.Domain + "/open/api/v2/order/open_orders"
	params := map[string]string{
		"symbol": symbol,
	}
	var resp ResponseOpenOrders
	if err := mxc.requestGet(url, params, &resp); err == nil {
		log.Printf("resp code = %d", resp.Code)
		return resp.Data, nil
	} else {
		return nil, err
	}
}

func (mxc *MXC) Deals(symbol, limit, start string) ([]Deal, error) {
	url := mxc.Domain + "/open/api/v2/order/deals"
	params := map[string]string{
		"symbol":     symbol,
		"limit":      limit,
		"start_time": start,
	}
	var resp ResponseDeals
	if err := mxc.requestGet(url, params, &resp); err == nil {
		return resp.Data, nil
	} else {
		return nil, err
	}
}

func (mxc *MXC) requestGet(url string, params map[string]string, result interface{}) error {
	params["api_key"] = mxc.Key
	params["req_time"] = fmt.Sprintf("%d", time.Now().Unix())
	signed := mxc.getSign(params)

	signedUrl := fmt.Sprintf("%s?%s", url, signed)

	resp, err := http.Get(signedUrl)
	if err != nil {
		return err
	}

	defer func() {
		if resp != nil {
			_ = resp.Body.Close()
		}
	}()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		//log.Printf("error: %s", err)
		return err
	}
	if err = json.Unmarshal(data, result); err != nil {
		log.Printf("raw response: %s", string(data))
	}
	return err
}

func (mxc *MXC) getSign(params map[string]string) string {
	key := []byte(mxc.Secret)
	mac := hmac.New(sha256.New, key)
	var keys []string
	for k, _ := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var paramSlice []string
	for _, k := range keys {
		paramSlice = append(paramSlice, fmt.Sprintf("%s=%s", k, params[k]))
	}
	paramStr := strings.Join(paramSlice, "&")
	mac.Write([]byte(paramStr))
	return fmt.Sprintf("%s&sign=%x", paramStr, mac.Sum(nil))
}
