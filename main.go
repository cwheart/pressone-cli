package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	viper "github.com/spf13/viper"
)

var (
	config Config
	apiURL = "https://dev.press.one/api/v2/nft/collections"
)

type auctionConfig struct {
	UUID            string `mapstructure:"uuid"`
	ContractAddress string `mapstructure:"contract_address"`
	TokenID         string `mapstructure:"token_id"`
}

type appConfig struct {
	AuthorizationToken string `mapstructure:"authorization_token"`
	BigONEUrl          string `mapstructure:"bigone_url"`
	AssetHost          string `mapstructure:"asset_host"`
}

type Config struct {
	Auctions []*auctionConfig `mapstructure:"auctions"`
	App      *appConfig       `mapstructure:"app"`
}

type goods struct {
	GUID     string `json:"guid"`
	Template struct {
		Attachments []struct {
			Path string `json:"path"`
		} `json:"attachments"`
	} `json:"template"`
}

type auction struct {
	Asset struct {
		UUID   string `json:"uuid"`
		Symbol string `json:"symbol"`
	} `json:"asset"`
}

type auctionResponse struct {
	Data struct {
		Auction *auction `json:"auction"`
		Goods   *goods   `json:"goods"`
	} `json:"data"`
}

type bid struct {
	User struct {
		GUID     string `json:"guid"`
		Nickname string `json:"nickname"`
	} `json:"user"`
	Price     string `json:"price"`
	CreatedAt string `json:"created_at"`
}

type bidResponse struct {
	Data struct {
		Bids []*bid `json:"bids"`
	} `json:"data"`
}

func initConfig() {
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal(err)
	}

	err = viper.Unmarshal(&config)
	if err != nil {
		log.Fatalf("load config yaml error: %v", err)
	}
}

func getAuction(a *auctionConfig) (*auction, *goods, error) {
	res, err := http.Get(fmt.Sprintf("%s/api/nft/v1/auctions/%s/detail", config.App.BigONEUrl, a.UUID))
	if err != nil {
		log.Fatal("get auction detail error: %s", err)
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal("read auction detail response error: %s", err)
	}
	auctionRes := &auctionResponse{}
	return auctionRes.Data.Auction, auctionRes.Data.Goods, json.Unmarshal(body, &auctionRes)
}

func listBids(a *auctionConfig) ([]*bid, error) {
	res, err := http.Get(fmt.Sprintf("%s/api/nft/v1/auctions/%s/bids", config.App.BigONEUrl, a.UUID))
	if err != nil {
		log.Fatal("get auction detail error: %s", err)
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal("read auction detail response error: %s", err)
	}
	bidsRes := &bidResponse{}
	return bidsRes.Data.Bids, json.Unmarshal(body, &bidsRes)

}

func postData(auctionUUID string, body []byte) error {
	logrus.Debug(string(body))
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", config.App.AuthorizationToken))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	resBody, _ := ioutil.ReadAll(resp.Body)
	result := make(map[string]string)
	err = json.Unmarshal(resBody, &result)
	if err != nil {
		return err
	}
	log.Infof("%s, external url: %s", auctionUUID, result["showcaseUrl"])
	return nil
}

func submitAuction(a *auctionConfig) error {
	auction, goods, err := getAuction(a)
	if err != nil {
		log.Fatal("get auction detail error: %s", err)
	}
	bids, err := listBids(a)
	if err != nil {
		log.Fatal("list bids error: %s", err)
	}
	attachments := make([]map[string]string, len(goods.Template.Attachments))
	for i, attattachment := range goods.Template.Attachments {
		attachments[i] = map[string]string{
			"url": fmt.Sprintf("%s/%s", config.App.AssetHost, attattachment.Path),
		}
	}
	collectible := map[string]interface{}{
		"uuid":             goods.GUID,
		"contract_address": a.ContractAddress,
		"token_id":         a.TokenID,
		"media":            attachments,
	}
	data := make(map[string]interface{})
	data["digital_collectibles"] = collectible
	bidsReq := make([]map[string]interface{}, len(bids))
	for i, bid := range bids {
		bidsReq[i] = map[string]interface{}{
			"price": map[string]interface{}{
				"value": bid.Price,
				"unit": map[string]interface{}{
					"uuid":   auction.Asset.UUID,
					"symbol": auction.Asset.Symbol,
				},
			},
			"holder": map[string]string{
				"uuid":     bid.User.GUID,
				"nickname": bid.User.Nickname,
			},
			"bid_at": bid.CreatedAt,
		}
	}
	data["bids"] = bidsReq
	body, err := json.Marshal(data)
	if err != nil {
		log.Fatal("invalid json data: %s", err)
	}
	if err = postData(a.UUID, body); err != nil {
		log.Fatal("post data error: %s", err)
	}
	return nil
}

func main() {
	initConfig()
	for _, a := range config.Auctions {
		if err := submitAuction(a); err != nil {
			log.Fatal("submit auction  error: %s", err)
		}
	}

}
