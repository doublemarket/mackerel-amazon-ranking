package main

import (
	"fmt"
	"github.com/mackerelio/mackerel-client-go"
	"github.com/ngs/go-amazon-product-advertising-api/amazon"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"strings"
	"time"
)

type Config struct {
	Region       amazon.Region `yaml:"amazon_region"`
	AccessKey    string        `yaml:"amazon_access_key"`
	AccessSecret string        `yaml:"amazon_access_secret"`
	Account      string        `yaml:"amazon_account"`
	MackerelKey  string        `yaml:"mackerel_key"`
	ASINs        []string      `yaml:"asins"`
	Prefix       string        `yaml:"metric_prefix"`
}

func retry(attempts int, sleep time.Duration, callback func() error) (err error) {
	for i := 0; ; i++ {
		err = callback()
		if err == nil {
			return
		}

		if i >= (attempts - 1) {
			break
		}

		time.Sleep(sleep)

		//        log.Println("Retrying", i+1, "times after error:", err)
	}
	return fmt.Errorf("After %d attempts, last error: %s", attempts, err)
}

func main() {
	buf, err := ioutil.ReadFile("./config.yaml")
	if err != nil {
		log.Fatalf("Unable to open the config yaml %v", err)
	}

	var config Config
	err = yaml.Unmarshal(buf, &config)
	if err != nil {
		log.Fatalf("Unable to parse the config yaml %v", err)
	}

	aclient, err := amazon.New(config.AccessKey, config.AccessSecret, config.Account, config.Region)
	if err != nil {
		log.Fatalf("Unable to setup an Amazon client %v", err)
	}
	time.Sleep(5 * time.Second)

	var ilres *amazon.ItemLookupResponse
	err = retry(10, 3*time.Second, func() (err error) {
		ilres, err = aclient.ItemLookup(amazon.ItemLookupParameters{
			ResponseGroups: []amazon.ItemLookupResponseGroup{amazon.ItemLookupResponseGroupSalesRank},
			ItemIDs:        config.ASINs,
		}).Do()
		return
	})
	if err != nil {
		if !strings.Contains(err.Error(), "NoExactMatches") {
			log.Fatalf("Unable to search an Item %v", err)
		}
	} else {
		mclient := mackerel.NewClient(config.MackerelKey)

		for _, item := range ilres.Items.Item {
			log.Printf("%s %v\n", item.ASIN, item.SalesRank)
			mclient.PostServiceMetricValues("Books", []*mackerel.MetricValue{
				&mackerel.MetricValue{
					Name:  config.Prefix + "." + item.ASIN,
					Time:  time.Now().Unix(),
					Value: item.SalesRank,
				},
			})
		}
	}
}
