package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/net/proxy"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// UsageSubscription 订阅信息
type UsageSubscription struct {
	Object             string  `json:"object"`
	HasPaymentMethod   bool    `json:"has_payment_method"`
	Canceled           bool    `json:"canceled"`
	CanceledAt         any     `json:"canceled_at"`
	Delinquent         any     `json:"delinquent"`
	AccessUntil        int     `json:"access_until"` // key到期时间
	SoftLimit          int     `json:"soft_limit"`
	HardLimit          int     `json:"hard_limit"`
	SystemHardLimit    int     `json:"system_hard_limit"`
	SoftLimitUsd       float64 `json:"soft_limit_usd"`
	HardLimitUsd       float64 `json:"hard_limit_usd"` // 总计可使用金额
	SystemHardLimitUsd float64 `json:"system_hard_limit_usd"`
	Plan               struct {
		Title string `json:"title"`
		Id    string `json:"id"` // free 免费
	} `json:"plan"`
	AccountName     string `json:"account_name"` // 账户名称
	PoNumber        any    `json:"po_number"`
	BillingEmail    any    `json:"billing_email"`
	TaxIds          any    `json:"tax_ids"`
	BillingAddress  any    `json:"billing_address"`
	BusinessAddress any    `json:"business_address"`
}

// UsageDailyList 每日使用情况列表
type UsageDailyList struct {
	Object     string `json:"object"`
	DailyCosts []struct {
		Timestamp float64 `json:"timestamp"`
		LineItems []struct {
			Name string  `json:"name"`
			Cost float64 `json:"cost"`
		} `json:"line_items"`
	} `json:"daily_costs"`
	TotalUsage float64 `json:"total_usage"` //  505.79316000000006
}

// UsageInfo 使用情况
type UsageInfo struct {
	AccessUntil        string  `json:"access_until"`         // key 到期时间
	HardLimitUsd       float64 `json:"hard_limit_usd"`       // 总计金额
	AccountName        string  `json:"account_name"`         // 账户名称
	UsedAmountUsd      float64 `json:"used_amount_usd"`      // 已使用金额
	RemainingAmountUsd float64 `json:"remaining_amount_usd"` // 剩余可用金额
}

// GetUsageByKey 获取key的使用情况
func GetUsageByKey(key string, proxyEnable bool, proxyHttp string, proxySocket5 string) (*UsageInfo, error) {
	reqUrl := "https://api.openai.com/v1/dashboard/billing/subscription"
	method := "GET"

	client := &http.Client{
		Timeout: 20 * time.Second,
	}
	if proxyEnable {
		if proxyHttp != "" {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(&url.URL{Host: proxyHttp}),
			}

		} else if proxySocket5 != "" {
			dialer, err := proxy.SOCKS5("tcp", proxySocket5, nil, proxy.Direct)
			if err != nil {
				fmt.Println(err)
				return nil, err
			}
			client.Transport = &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return dialer.Dial(network, addr)
				},
			}
		}
	}
	req, err := http.NewRequest(method, reqUrl, nil)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+key)

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	//parse body to UsageSubscription
	var usage UsageSubscription
	err = json.Unmarshal(body, &usage)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	//parse UsageSubscription to UsageInfo
	var usageInfo UsageInfo
	usageInfo.HardLimitUsd = usage.HardLimitUsd
	usageInfo.AccountName = usage.AccountName
	//usageInfo.UsedAmountUsd = usage.HardLimitUsd - usage.SoftLimitUsd
	//usageInfo.RemainingAmountUsd = usage.SoftLimitUsd

	//  usage.AccessUntil is a timestamp
	startTime := ""
	endTime := ""
	if time.Now().Local().Unix() > int64(usage.AccessUntil) {
		endTime = time.Unix(int64(usage.AccessUntil), 0).Format("2006-01-02")
		usageInfo.AccessUntil = endTime + "-已过期"
		// 开始时间，在到期时间的前三个月
		startTime = time.Unix(int64(usage.AccessUntil), 0).AddDate(0, -3, 0).Format("2006-01-02")
	} else {
		usageInfo.AccessUntil = time.Unix(int64(usage.AccessUntil), 0).Format("2006-01-02 15:04:05")
		endTime = time.Now().Local().Format("2006-01-02")
		// 开始时间，在当前时间的前三个月
		startTime = time.Now().Local().AddDate(0, -3, 0).Format("2006-01-02")
	}

	//2023-04-01
	reqUrl = fmt.Sprintf("https://api.openai.com/v1/dashboard/billing/usage?start_date=%s&end_date=%s", startTime, endTime)
	req, err = http.NewRequest(method, reqUrl, nil)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+key)

	res, err = client.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(res.Body)

	body, err = io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	fmt.Println(string(body))
	// parse body to UsageDailyList
	var usageDailyList UsageDailyList
	err = json.Unmarshal(body, &usageDailyList)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// parse UsageDailyList to UsageInfo
	usageInfo.UsedAmountUsd, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", usageDailyList.TotalUsage/100.00), 64)
	usageInfo.RemainingAmountUsd = usageInfo.HardLimitUsd - usageInfo.UsedAmountUsd
	return &usageInfo, nil
}
