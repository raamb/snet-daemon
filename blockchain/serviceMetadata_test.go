package blockchain

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"math/big"
	"strings"
	"testing"
)

var testJsonData = "{   \"version\": 1,   \"display_name\": \"Example1\",   \"encoding\": \"grpc\",   \"service_type\": \"grpc\",   \"payment_expiration_threshold\": 40320,   \"model_ipfs_hash\": \"Qmdiq8Hu6dYiwp712GtnbBxagyfYyvUY1HYqkH7iN76UCc\",   \"mpe_address\": \"0x7E6366Fbe3bdfCE3C906667911FC5237Cc96BD08\",   \"groups\": [     {       \"endpoints\": [\"http://34.344.33.1:2379\",\"http://34.344.33.1:2389\"],       \"group_id\": \"88ybRIg2wAx55mqVsA6sB4S7WxPQHNKqa4BPu/bhj+U=\",\"group_name\": \"default_group\",       \"pricing\": [         {           \"price_model\": \"fixed_price\",           \"price_in_cogs\": 2         },          {         \"package_name\": \"example_service\",         \"price_model\": \"fixed_price_per_method\",         \"default\":true,         \"details\": [           {             \"service_name\": \"Calculator\",             \"method_pricing\": [               {                 \"method_name\": \"add\",                 \"price_in_cogs\": 2               },               {                 \"method_name\": \"sub\",                 \"price_in_cogs\": 1               },               {                 \"method_name\": \"div\",                 \"price_in_cogs\": 2               },               {                 \"method_name\": \"mul\",                 \"price_in_cogs\": 3               }             ]           },           {             \"service_name\": \"Calculator2\",             \"method_pricing\": [               {                 \"method_name\": \"add\",                 \"price_in_cogs\": 2               },               {                 \"method_name\": \"sub\",                 \"price_in_cogs\": 1               },               {                 \"method_name\": \"div\",                 \"price_in_cogs\": 3               },               {                 \"method_name\": \"mul\",                 \"price_in_cogs\": 2               }             ]           }         ]       }]     },     {       \"endpoints\": [\"http://97.344.33.1:2379\",\"http://67.344.33.1:2389\"],       \"group_id\": \"99ybRIg2wAx55mqVsA6sB4S7WxPQHNKqa4BPu/bhj+U=\",       \"pricing\": [         {         \"package_name\": \"example_service\",         \"price_model\": \"fixed_price_per_method\",         \"details\": [           {             \"service_name\": \"Calculator\",             \"method_pricing\": [               {                 \"method_name\": \"add\",                 \"price_in_cogs\": 2               },               {                 \"method_name\": \"sub\",                 \"price_in_cogs\": 1               },               {                 \"method_name\": \"div\",                 \"price_in_cogs\": 2               },               {                 \"method_name\": \"mul\",                 \"price_in_cogs\": 3               }             ]           },           {             \"service_name\": \"Calculator2\",             \"method_pricing\": [               {                 \"method_name\": \"add\",                 \"price_in_cogs\": 2               },               {                 \"method_name\": \"sub\",                 \"price_in_cogs\": 1               },               {                 \"method_name\": \"div\",                 \"price_in_cogs\": 3               },               {                 \"method_name\": \"mul\",                 \"price_in_cogs\": 2               }             ]           }         ]       }]     }   ] } "

func TestAllGetterMethods(t *testing.T) {
	println(testJsonData)
	metaData, err := InitServiceMetaDataFromJson(testJsonData)
	assert.Equal(t, err, nil)

	assert.Equal(t, metaData.GetVersion(), 1)
	assert.Equal(t, metaData.GetDisplayName(), "Example1")
	assert.Equal(t, metaData.GetServiceType(), "grpc")
	assert.Equal(t, metaData.GetWireEncoding(), "grpc")
	assert.Nil(t,metaData.GetDefaultPricing().PriceInCogs)
	assert.Equal(t, metaData.GetDefaultPricing().PricingDetails[0].MethodPricing[0].PriceInCogs, big.NewInt(2))
	assert.Equal(t, metaData.GetMpeAddress(), common.HexToAddress("0x7E6366Fbe3bdfCE3C906667911FC5237Cc96BD08"))



}


func TestInitServiceMetaDataFromJson(t *testing.T) {
	//Parse Bad JSON
	  _, err := InitServiceMetaDataFromJson(strings.Replace(testJsonData, "{", "", 1))
	if err != nil {
		assert.Equal(t, err.Error(), "invalid character ':' after top-level value")
	}



}

func TestReadServiceMetaDataFromLocalFile(t *testing.T) {


	metadata, err := ReadServiceMetaDataFromLocalFile("../service_metadata.json")
	assert.Equal(t, err, nil)
	assert.Equal(t, metadata.Version, 1)

}
