package service

import (
	"fmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
)

type Runner struct {
	Org1Name	string
	Org2Name 	string
	Org1AdminUser	string
	Org2AdminUser	string
	Org1User	string
	Org2User	string
	ChannelID	string
	CCPath		string
	sdk			*fabsdk.FabricSDK
	setup		*BaseSetupImpl
	installCC 	bool
	ChainCodeID	string
}

func New() *Runner{
	r := Runner{
		Org1Name:      GetSDKOrgs()[0],
		Org2Name:      GetSDKOrgs()[1],
		Org1AdminUser: GetSDKAdmins()[0],
		Org2AdminUser: GetSDKAdmins()[1],
		Org1User:      GetSDKUsers()[0],
		Org2User:      GetSDKUsers()[1],
		ChannelID:     GetSDKChannelID(),
		CCPath:        GetChainCodePath(),
		/*sdk:           nil,
		setup:         nil,
		//installCC:     false,
		ChainCodeID:   "",*/
	}
	return &r
}


func NewWithCC()  *Runner{
	r := New()
	r.installCC =true

	return  r
}

func (r *Runner) Initialize()  {
	r.setup = &BaseSetupImpl{
		//Identity:                msp.IdentityConfig{},
		//Targets:                 nil,
		//ConfigFile:              "",
		OrgID:                   r.Org1Name,
		ChannelID:               r.ChannelID,
		ChannelConfigFile:       GetChannelConfigPath(r.ChannelID+".tx"),
		//ledgerClient:            nil,
		//ChClient:                nil,
		//ChainCodeID:             "",
		//OrgChannelClientContext: nil,
	}

	sdk,err := fabsdk.New(FetchConfigBackend())
	if err!=nil{
		panic(fmt.Sprintf("Failed to create new SDK: %s", err))
	}
	r.sdk =sdk
	// Delete all private keys from the crypto suite store
	// and users from the user store
	CleanupUserData(sdk)

	if err := r.setup.Initialize(sdk);err != nil{
		panic(err.Error())
	}

	if r.installCC {
		r.ChainCodeID = GenerateID(false)
		if err := PrepareCC(sdk,fabsdk.WithUser("Admin"),r.setup.OrgID,r.ChainCodeID);err !=nil{
			panic(fmt.Sprintf("PrepareCC return error: %s", err))
		}
	}
}

//SDK returns the instantiated SDK instance.panics if nil.
func (r *Runner) GetSDK() *fabsdk.FabricSDK {
	if r.sdk == nil{
		panic("SDK not instantiated")
	}
	return r.sdk
}
// TestSetup returns the integration test setup.
func (r *Runner) GetSetup() *BaseSetupImpl {
	return r.setup
}
// ChaincodeID returns the generated chaincode ID for example CC.
func (r *Runner) GetChaincodeID() string {
	return r.ChainCodeID
}