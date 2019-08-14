package service

import (
	"fmt"
	"github.com/fjl/go-couchdb"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	fabApi "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config/lookup"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/cryptosuite"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/fabric-sdk-go/pkg/msp"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"math/rand"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	adminUser       = "Admin"
	ordererOrgName  = "OrdererOrg"
	ordererEndpoint = "orderer.example.com"

	minUnicodeRuneValue   = 0            //U+0000
	maxUnicodeRuneValue   = utf8.MaxRune //U+10FFFF - maximum (and unallocated) code point
	compositeKeyNamespace = "\x00"
)

//orders
func GetSDKOrders()	[]string{
	return viper.GetStringSlice("spi.sdk.orders")
}

//orgs
func GetSDKOrgs()	[]string{
	return viper.GetStringSlice("spi.sdk.orgs")
}

//admins
func GetSDKAdmins()	[]string{
	return viper.GetStringSlice("spi.sdk.adminUsers")
}

//user
func GetSDKUsers() []string {
	return viper.GetStringSlice("spi.sdk.users")
}

//channel ID
func GetSDKChannelID() string {
	return viper.GetString("spi.sdk.channelID")
}

//chaincode path
func GetChainCodePath() string {
	return path.Join(PROJECT_NAME,viper.GetString("spi.chaincode.path"))
}

//GetChainCode name
func GetChainCodeName()string{
	return viper.GetString("spi.chaincode.name")
}
//chaincode version
func GetChainCodeVersion() string{
	return viper.GetString("spi.chaincode.version")
}

func GetChannelConfig() string {
	return viper.GetString("spi.sdk.channelConfig")
}

//from core config net
func GetSDKConfigFile() string {
	return viper.GetString("spi.sdk.configPath")
}
//localEntityMatcher
func GetLocalEntityMatcher() string {
	return viper.GetString("spi.sdk.localEntityMatcher")
}

//CleanupUserData removes user data
func CleanupUserData(sdk *fabsdk.FabricSDK)  {
	var keyStorePath,credentialStorePath string

	configBackend,err := sdk.Config()
	if err!=nil{
		// if an error is returned from Config, it means configBackend was nil, in this case simply hard code
		// the keyStorePath and credentialStorePath to the default values
		// This case is mostly happening due to configless test that is not passing a ConfigProvider to the SDK
		// which makes configBackend = nil.
		// Since configless test uses the same config values as the default ones (config_test.yaml), it's safe to
		// hard code these paths here
		keyStorePath = "/tmp/msp/keystore"
		credentialStorePath = "/tmp/state-store"
	} else {
		cryptoSuiteConfig := cryptosuite.ConfigFromBackend(configBackend)
		identityConfig,err := msp.ConfigFromBackend(configBackend)
		if err!=nil{
			panic(fmt.Sprintf("CleanupUserData failed: %v", err))
		}

		keyStorePath = cryptoSuiteConfig.KeyStorePath()
		credentialStorePath = identityConfig.CredentialStorePath()
	}
	CleanupPath(keyStorePath)
	CleanupPath(credentialStorePath)
}

func CleanupPath(storePath string)  {
	err := os.RemoveAll(storePath)
	if err != nil{
		panic(fmt.Sprintf("Cleaning up directory '%s' failed: %v", storePath, err))
	}
}

// OrgTargetPeers determines peer endpoints for orgs
func OrgTargetPeers(orgs []string, configBackend ...core.ConfigBackend) ([]string, error) {
	networkConfig := fabApi.NetworkConfig{}
	err := lookup.New(configBackend...).UnmarshalKey("organizations", &networkConfig.Organizations)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get organizations from config ")
	}

	var peers []string
	for _, org := range orgs {
		orgConfig, ok := networkConfig.Organizations[strings.ToLower(org)]
		if !ok {
			continue
		}
		peers = append(peers, orgConfig.Peers...)
	}
	return peers, nil
}

//InitializeChannel ...
func InitializeChannel(sdk *fabsdk.FabricSDK,orgID string,req resmgmt.SaveChannelRequest,targets []string)error  {
	joinedTargets, err := FilterTargetsJoinedChannel(sdk, orgID, req.ChannelID, targets)
	if err != nil {
		return errors.WithMessage(err, "checking for joined targets failed")
	}

	if len(joinedTargets) != len(targets) {
		_, err := CreateChannel(sdk, req)
		if err != nil {
			return errors.Wrapf(err, "create channel failed")
		}

		_, err = JoinChannel(sdk, req.ChannelID, orgID, targets)
		if err != nil {
			return errors.Wrapf(err, "join channel failed")
		}
	}
	return nil
}

// FilterTargetsJoinedChannel filters targets to those that have joined the named channel.
func FilterTargetsJoinedChannel(sdk *fabsdk.FabricSDK, orgID string, channelID string, targets []string) ([]string, error) {
	var joinedTargets []string

	//prepare context
	clientContext := sdk.Context(fabsdk.WithUser(adminUser), fabsdk.WithOrg(orgID))

	rc, err := resmgmt.New(clientContext)
	if err != nil {
		return nil, errors.WithMessage(err, "failed getting admin user session for org")
	}

	for _, target := range targets {
		// Check if primary peer has joined channel
		alreadyJoined, err := HasPeerJoinedChannel(rc, target, channelID)
		if err != nil {
			return nil, errors.WithMessage(err, "failed while checking if primary peer has already joined channel")
		}
		if alreadyJoined {
			joinedTargets = append(joinedTargets, target)
		}
	}
	return joinedTargets, nil
}

// CreateChannel attempts to save the named channel.
func CreateChannel(sdk *fabsdk.FabricSDK, req resmgmt.SaveChannelRequest) (bool, error) {

	//prepare context
	clientContext := sdk.Context(fabsdk.WithUser(adminUser), fabsdk.WithOrg(ordererOrgName))

	// Channel management client is responsible for managing channels (create/update)
	resMgmtClient, err := resmgmt.New(clientContext)
	if err != nil {
		return false, errors.WithMessage(err, "Failed to create new channel management client")
	}

	// Create channel (or update if it already exists)
	if _, err = resMgmtClient.SaveChannel(req, resmgmt.WithRetry(retry.DefaultResMgmtOpts), resmgmt.WithOrdererEndpoint(ordererEndpoint)); err != nil {
		return false, err
	}

	return true, nil
}

// JoinChannel attempts to save the named channel.
func JoinChannel(sdk *fabsdk.FabricSDK, name, orgID string, targets []string) (bool, error) {
	//prepare context
	clientContext := sdk.Context(fabsdk.WithUser(adminUser), fabsdk.WithOrg(orgID))

	// Resource management client is responsible for managing resources (joining channels, install/instantiate/upgrade chaincodes)
	resMgmtClient, err := resmgmt.New(clientContext)
	if err != nil {
		return false, errors.WithMessage(err, "Failed to create new resource management client")
	}

	if err := resMgmtClient.JoinChannel(
		name,
		resmgmt.WithRetry(retry.DefaultResMgmtOpts),
		resmgmt.WithTargetEndpoints(targets...),
		resmgmt.WithOrdererEndpoint(ordererEndpoint)); err != nil {
		return false, nil
	}
	return true, nil
}

// HasPeerJoinedChannel checks whether the peer has already joined the channel.
// It returns true if it has, false otherwise, or an error
func HasPeerJoinedChannel(client *resmgmt.Client, target string, channel string) (bool, error) {
	foundChannel := false
	response, err := client.QueryChannels(resmgmt.WithTargetEndpoints(target), resmgmt.WithRetry(retry.DefaultResMgmtOpts))
	if err != nil {
		return false, errors.WithMessage(err, "failed to query channel for peer")
	}
	for _, responseChannel := range response.Channels {
		if responseChannel.ChannelId == channel {
			foundChannel = true
		}
	}

	return foundChannel, nil
}

// GenerateRandomID generates random ID
func GenerateRandomID() string {
	return randomString(10)
}


// Utility to create random string of strlen length
func randomString(strlen int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, strlen)
	seed := rand.NewSource(time.Now().UnixNano())
	rnd := rand.New(seed)
	for i := 0; i < strlen; i++ {
		result[i] = chars[rnd.Intn(len(chars))]
	}
	return string(result)
}

// chaincode composite key
func validateCompositeKeyAttribute(str string) error {
	if !utf8.ValidString(str) {
		return fmt.Errorf("Not a valid utf8 string: [%s].", str)
	}
	for index, runeValue := range str {
		if runeValue == minUnicodeRuneValue || runeValue == maxUnicodeRuneValue {
			return fmt.Errorf(`Input contain unicode %#U starting at position [%d]. %#U and %#U are not allowed in the input attribute of a composite key`,
				runeValue, index, minUnicodeRuneValue, maxUnicodeRuneValue)
		}
	}
	return nil
}
// chaincode composite key
func createCompositeKey(objectType string, attributes []string) (string, error) {
	if err := validateCompositeKeyAttribute(objectType); err != nil {
		return "", err
	}
	ck := compositeKeyNamespace + objectType + string(minUnicodeRuneValue)
	for _, att := range attributes {
		if err := validateCompositeKeyAttribute(att); err != nil {
			return "", err
		}
		ck += att + string(minUnicodeRuneValue)
	}
	return ck, nil
}

// getKV ...
func getKV(bsi *BaseSetupImpl, attributes []string, keyPre string) (string, error) {
	key, err := createCompositeKey(keyPre, attributes)
	if err != nil {
		return "", err
	}

	r, err := bsi.Find("query",key)
	fmt.Printf("query chaincode key=%s; result=%s", key, r)

	return r, err
}

var CouchClient *couchdb.Client
var CouchDb *couchdb.DB

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
//initcouchdb
func InitCouchdb() (err error)  {
	/*rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("nothing to see here, move along")
	})*/
	cacheAddress := "http://0.0.0.0:12984/"
	fmt.Println("cacheAddress",cacheAddress)
	CouchClient,err = couchdb.NewClient(cacheAddress,nil)
	if err!=nil{
		fmt.Println("NewClient failed",err)
		return
	}
	err = CouchClient.Ping()
	if err!=nil{
		fmt.Println("Connect CouchDb  failed",err)
		return
	}

	CouchDb,err = CouchClient.CreateDB("carcache")
	if err!=nil{
		fmt.Printf("create DB failed",err)
		return
	}
	fmt.Println("success create CarCache DB",CouchDb)
	return
}