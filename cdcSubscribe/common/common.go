package common

import (
	"time"

	"SalesforceGit/cdcSubscribe/proto"
)

type DateFormat struct {
	Timezone   string
	DateFormat string
	TimeFormat string
}

var DateFormatValues DateFormat

type Topic struct {
	ChannelName         string
	CurrentlySubscribed bool
	Error               string
}

var AccessToken string

type IntegrationUserValues struct {
	Username      string  `bson:"Username" json:"Username"`
	SalesforceKey string  `bson:"SalesforceKey" json:"SalesforceKey"`
	Topics        []Topic `bson:"Topics" json:"Topics"`
}

var Schemas map[string][]FieldsNamesFromSchema

var TestSchemas map[string][]Field

type Field struct {
	Name string
	Doc  Doc
}

var DocTypes map[string][]Doc

type Doc struct {
	DocName string
	Name    string
	Fields  []string
}

// Schemas = make(map[string][]FieldsNamesFromSchema)
//var FieldsNames []FieldsNamesFromSchema

var Schema map[string]string

type FieldsNamesFromSchema struct {
	Name      string   `json:"name"`
	SubFields []string `json:"subfields"`
}

var (
	// topic and subscription-related variables
	ReplayPreset        = proto.ReplayPreset_LATEST
	ReplayId     []byte = nil
	Appetite     int32  = 5

	// gRPC server variables
	GRPCEndpoint    = "api.pubsub.salesforce.com:7443"
	GRPCDialTimeout = 5 * time.Second
	GRPCCallTimeout = 5 * time.Second

	// OAuth server variables
	OAuthEndpoint    = "https://login.salesforce.com"
	OAuthDialTimeout = 5 * time.Second
)
