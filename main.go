package main

import (
	"fmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/ledger"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/logging"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/spf13/viper"
	"myfabricnet/service"
	"net/http"
)

var (
	mainSDK *fabsdk.FabricSDK
	mainSetup *service.BaseSetupImpl
	err error
	logger   *logging.Logger
)

func main(){
	fmt.Println("myfabricnet start main")
	//init core.yml
	//about chaincode
	//about sdk
	//about channel user
	if err = service.InitConfig();err != nil{
		panic(fmt.Errorf("fatal errror when initializing config : %s",err))
	}
	fmt.Println("InitConfig core.yml success")
	//init couchdbCache
	err = service.InitCouchdb()
	if err!=nil{
		fmt.Println("Init CouchDb Cache failed")
	}
	//create runner -org chaincode channel
	//set initial value
	r := service.NewWithCC()

	//install chaincode
	//
	//r.sdk r.setup

	r.Initialize()
	fmt.Printf("finish init %s \n",service.PROJECT_NAME)

	//set the channel context
	//creat new Setup
	service.DefSetup= &service.BaseSetupImpl{}
	mainSDK = r.GetSDK()
	mainSetup = r.GetSetup()
	service.DefSetup.ChainCodeID = r.GetChaincodeID()
	service.DefSetup.ChannelID = mainSetup.ChannelID
	service.DefSetup.OrgChannelClientContext = mainSDK.ChannelContext(mainSetup.ChannelID,fabsdk.WithUser(r.Org1User),fabsdk.WithOrg(r.Org1Name))
	service.DefSetup.ChClient,err = channel.New(service.DefSetup.OrgChannelClientContext)
	if err!=nil{
		logger.Errorf("ledger.New:%s",err)
	}

	//ledger
	service.DefSetup.LedgerClient,err = ledger.New(service.DefSetup.OrgChannelClientContext)
	if err != nil {
		logger.Errorf("ledger.New: %s", err)
	}

	//router register
	router := service.BuildRouter()
	restAddress := viper.GetString("spi.rest.address")
	fmt.Printf("listen at the address : %s \n",restAddress)

	err = http.ListenAndServe(restAddress,router)
	if err!=nil{
		logger.Errorf("ListenAndServer: %s",err)
	}
}