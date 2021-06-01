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
	apiURL             = "https://dev.press.one/api/v2/nft/collections"
	assetHost          string
	contractAddress    string
	authorizationToken string
	bigoneURL          string
	auctionUUID        string
	tokenID            string
)

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
		log.Fatalf("load config yaml error: %v", err)
	}
	authorizationToken = viper.GetString("app.authorization_token")
	bigoneURL = viper.GetString("app.bigone_url")
	assetHost = viper.GetString("app.asset_host")

	auctionUUID = viper.GetString("auction.uuid")
	contractAddress = viper.GetString("auction.contract_address")
	tokenID = viper.GetString("auction.token_id")
}

func getAuction() (*auction, *goods, error) {
	res, err := http.Get(fmt.Sprintf("%s/api/nft/v1/auctions/%s/detail", bigoneURL, auctionUUID))
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

func listBids() ([]*bid, error) {
	res, err := http.Get(fmt.Sprintf("%s/api/nft/v1/auctions/%s/bids", bigoneURL, auctionUUID))
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

func postData(body []byte) error {
	logrus.Info(string(body))
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", authorizationToken))
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
	log.Infof("external url: %s", result["showcaseUrl"])
	return nil
}

func main() {
	initConfig()
	auction, goods, err := getAuction()
	if err != nil {
		log.Fatal("get auction detail error: %s", err)
	}
	bids, err := listBids()
	if err != nil {
		log.Fatal("list bids error: %s", err)
	}
	attachments := make([]map[string]string, len(goods.Template.Attachments))
	for i, attattachment := range goods.Template.Attachments {
		attachments[i] = map[string]string{
			"url": fmt.Sprintf("%s/%s", assetHost, attattachment.Path),
		}
	}
	collectible := map[string]interface{}{
		"uuid":             goods.GUID,
		"contract_address": contractAddress,
		"token_id":         tokenID,
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
	if err = postData(body); err != nil {
		log.Fatal("post data error: %s", err)
	}
}
