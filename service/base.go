package service

import (
	"encoding/base64"
	"fmt"
	"github.com/golang/protobuf/proto"
	google_protobuf "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/ledger"
	mspclient "github.com/hyperledger/fabric-sdk-go/pkg/client/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/status"
	contextAPI "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	fabAPI "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	contextImpl "github.com/hyperledger/fabric-sdk-go/pkg/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/resource"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/fabric-sdk-go/pkg/util/test"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/common/cauthdsl"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/rwsetutil"
	cb "github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/protos/peer"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/protos/utils"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go/build"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const PROJECT_NAME  =  "myfabricnet"

type BaseSetupImpl struct {
	Identity	msp.Identity
	Targets		[]string
	ConfigFile	string
	OrgID		string
	ChannelID	string
	ChannelConfigFile string

	LedgerClient	*ledger.Client
	ChClient		*channel.Client
	ChainCodeID		string
	OrgChannelClientContext		contextAPI.ChannelProvider
}
// OrgContext provides SDK client context for a given org
type OrgContext struct {
	OrgID                string
	CtxProvider          contextAPI.ClientProvider
	SigningIdentity      msp.SigningIdentity
	ResMgmt              *resmgmt.Client
	Peers                []fabAPI.Peer
	AnchorPeerConfigFile string
}

// Initial B values for CC
const (
	CCInitB    = "200"
	CCUpgradeB = "400"
	keyExp            = "key-%s-%s"
)

// CC init and upgrade args
var initArgs = [][]byte{[]byte("init"), []byte("a"), []byte("100"), []byte("b"), []byte(CCInitB)}
var resetArgs = [][]byte{[]byte("a"), []byte("100"), []byte("b"), []byte(CCInitB)}

func InitConfig()(err error){
	//viper init
	//getConfigPathAndName
	viper.AddConfigPath("./")
	viper.SetConfigName("core")

	viper.SetEnvPrefix(PROJECT_NAME)
	viper.AutomaticEnv()  //V.automaticEnvApplied=true
	replacer := strings.NewReplacer(".","-")
	viper.SetEnvKeyReplacer(replacer)
	err = viper.ReadInConfig()
	if err!=nil{
		return fmt.Errorf("fatal error config file: %s ", err)
	}
	return nil
}

func GetChannelConfigPath(filename string) string {
	return path.Join(goPath(),"src",PROJECT_NAME,GetChannelConfig(),filename)
}

func goPath() string{
	gpDefault := build.Default.GOPATH
	gps := filepath.SplitList(gpDefault)

	return gps[0]
}

func CCQueryByKeys(fcn string,key string,key2 string) [][]byte {
	return [][]byte{[]byte(fcn),[]byte(key), []byte(key2)}
}
// CCQueryArgs returns  cc query args
func CCFindArgs(fcn string,key string) [][]byte {
	return [][]byte{[]byte(fcn), []byte(key)}
}
//CCTxSetArgs sets the given key value in cc
func CCTxSetArgs(key, value string) [][]byte {
	return [][]byte{[]byte("set"), []byte(key), []byte(value)}
}
//CCInitArgs returns  cc initialization args
func CCInitArgs() [][]byte {
	return initArgs
}


// Initialize reads configuration from file and sets up client and channel
func (setup *BaseSetupImpl) Initialize(sdk *fabsdk.FabricSDK) error {
	mspClient, err := mspclient.New(sdk.Context(), mspclient.WithOrg(setup.OrgID))
	if err!=nil{
		return errors.WithMessage(err,"failed to get client")
	}
	adminIdentity, err := mspClient.GetSigningIdentity(GetSDKAdmins()[0])
	if err != nil {
		return  errors.WithMessage(err,"failed to get client context")
	}
	setup.Identity = adminIdentity

	var cfgBackends []core.ConfigBackend
	configBackend,err := sdk.Config()
	if err!=nil{
		//For some tests SDK may not have backend set, try with config file if backend is missing
		cfgBackends = append(cfgBackends,configBackend)
		return errors.Wrapf(err, "failed to get config backend from config: %s", err)

	}else {
		cfgBackends = append(cfgBackends, configBackend)
	}

	targets,err := OrgTargetPeers([]string{setup.OrgID},cfgBackends...)
	if err!=nil{
		return errors.Wrapf(err, "loading target peers from config failed")
	}
	setup.Targets=targets

	r,err := os.Open(setup.ChannelConfigFile)
	if err!=nil{
		return errors.Wrap(err,"openingchannel config file failed")

	}
	defer func() {
		if err := r.Close();err !=nil{
			test.Logf("close error %v",err)		}
	}()

	//create channel for tests
	req := resmgmt.SaveChannelRequest{ChannelID:setup.ChannelID,ChannelConfig:r,SigningIdentities:[]msp.SigningIdentity{adminIdentity}}
	if err = InitializeChannel(sdk,setup.OrgID,req,targets);err!=nil{
		return errors.WithMessage(err, "failed to initialize channel")
	}
	return nil
}

// InstallChaincodeWithOrgContexts installs the given chaincode to orgs
func InstallChaincodeWithOrgContexts(orgs []*OrgContext, ccPkg *resource.CCPackage, ccPath, ccID, ccVersion string) error {
	for _, orgCtx := range orgs {
		if err := InstallChaincode(orgCtx.ResMgmt, ccPkg, ccPath, ccID, ccVersion, orgCtx.Peers); err != nil {
			return errors.Wrapf(err, "failed to install chaincode to peers in org [%s]", orgCtx.OrgID)
		}
	}

	return nil
}

// InstallChaincode installs the given chaincode to the given peers
func InstallChaincode(resMgmt *resmgmt.Client, ccPkg *resource.CCPackage, ccPath, ccName, ccVersion string, localPeers []fabAPI.Peer) error {
	installCCReq := resmgmt.InstallCCRequest{Name: ccName, Path: ccPath, Version: ccVersion, Package: ccPkg}
	_, err := resMgmt.InstallCC(installCCReq, resmgmt.WithRetry(retry.DefaultResMgmtOpts))
	if err != nil {
		return err
	}

	installed, err := queryInstalledCC(resMgmt, ccName, ccVersion, localPeers)

	if err != nil {
		return err
	}

	if !installed {
		return errors.New("chaincode was not installed on all peers")
	}

	return nil
}

func queryInstalledCC(resMgmt *resmgmt.Client, ccName, ccVersion string, peers []fabAPI.Peer) (bool, error) {
	installed, err := retry.NewInvoker(retry.New(retry.TestRetryOpts)).Invoke(
		func() (interface{}, error) {
			ok, err := isCCInstalled(resMgmt, ccName, ccVersion, peers)
			if err != nil {
				return &ok, err
			}
			if !ok {
				return &ok, status.New(status.TestStatus, status.GenericTransient.ToInt32(), fmt.Sprintf("Chaincode [%s:%s] is not installed on all peers in Org1", ccName, ccVersion), nil)
			}
			return &ok, nil
		},
	)

	if err != nil {
		s, ok := status.FromError(err)
		if ok && s.Code == status.GenericTransient.ToInt32() {
			return false, nil
		}
		return false, errors.WithMessage(err, "isCCInstalled invocation failed")
	}

	return *(installed).(*bool), nil
}



func isCCInstalled(resMgmt *resmgmt.Client, ccName, ccVersion string, peers []fabAPI.Peer) (bool, error) {
	installedOnAllPeers := true
	for _, peerNode := range peers {
		resp, err := resMgmt.QueryInstalledChaincodes(resmgmt.WithTargets(peerNode))
		if err != nil {
			return false, errors.WithMessage(err, "querying for installed chaincodes failed")
		}

		found := false
		for _, ccInfo := range resp.Chaincodes {
			if ccInfo.Name == ccName && ccInfo.Version == ccVersion {
				found = true
				break
			}
		}
		if !found {
			installedOnAllPeers = false
		}
	}
	return installedOnAllPeers, nil
}
// InstantiateChaincode instantiates the given chaincode to the given channel
func InstantiateChaincode(resMgmt *resmgmt.Client, channelID, ccName, ccPath, ccVersion string, ccPolicyStr string, args [][]byte, collConfigs ...*cb.CollectionConfig) (resmgmt.InstantiateCCResponse, error) {
	ccPolicy, err := cauthdsl.FromString(ccPolicyStr)
	if err != nil {
		return resmgmt.InstantiateCCResponse{}, errors.Wrapf(err, "error creating CC policy [%s]", ccPolicyStr)
	}

	return resMgmt.InstantiateCC(
		channelID,
		resmgmt.InstantiateCCRequest{
			Name:       ccName,
			Path:       ccPath,
			Version:    ccVersion,
			Args:       args,
			Policy:     ccPolicy,
			CollConfig: collConfigs,
		},
		resmgmt.WithRetry(retry.DefaultResMgmtOpts),
	)
}

// GetDeployPath returns the path to the chaincode fixtures
func GetDeployPath() string {
	return goPath()
}
// DiscoverLocalPeers queries the local peers for the given MSP context and returns all of the peers. If
// the number of peers does not match the expected number then an error is returned.
func DiscoverLocalPeers(ctxProvider contextAPI.ClientProvider, expectedPeers int) ([]fabAPI.Peer, error) {
	ctx, err := contextImpl.NewLocal(ctxProvider)
	if err != nil {
		return nil, errors.Wrap(err, "error creating local context")
	}

	discoveredPeers, err := retry.NewInvoker(retry.New(retry.TestRetryOpts)).Invoke(
		func() (interface{}, error) {
			peers, err := ctx.LocalDiscoveryService().GetPeers()
			if err != nil {
				return nil, errors.Wrapf(err, "error getting peers for MSP [%s]", ctx.Identifier().MSPID)
			}
			if len(peers) < expectedPeers {
				return nil, status.New(status.TestStatus, status.GenericTransient.ToInt32(), fmt.Sprintf("Expecting %d peers but got %d", expectedPeers, len(peers)), nil)
			}
			return peers, nil
		},
	)
	if err != nil {
		return nil, err
	}

	return discoveredPeers.([]fabAPI.Peer), nil
}
type BlockchainInfo struct {
	Height            uint64 `json:"height"`
	CurrentBlockHash  string `json:"currentBlockHash"`
	PreviousBlockHash string `json:"previousBlockHash"`
}
func (setup *BaseSetupImpl)GetBlockchainInfo()(BlockchainInfo,error) {
	var blockchainInfo	BlockchainInfo
	b,err := setup.LedgerClient.QueryInfo()
	if err!=nil{
		return blockchainInfo,err
	}
	blockchainInfo.Height=b.BCI.Height
	blockchainInfo.CurrentBlockHash = base64.StdEncoding.EncodeToString(b.BCI.CurrentBlockHash)
	blockchainInfo.PreviousBlockHash = base64.StdEncoding.EncodeToString(b.BCI.PreviousBlockHash)

	return blockchainInfo, nil
}

type Block struct {
	Version           int           `protobuf:"bytes,1,opt,name=version" json:"version"`
	Timestamp         string        `protobuf:"bytes,2,opt,name=timestamp" json:"timestamp"`
	Transactions      []Transaction `protobuf:"bytes,3,opt,name=transactions" json:"transactions"`
	StateHash         string        `protobuf:"bytes,4,opt,name=stateHash" json:"stateHash"`
	PreviousBlockHash string        `protobuf:"bytes,5,opt,name=previousBlockHash" json:"previousBlockHash"`
	// NonHashData       LocalLedgerCommitTimestamp `protobuf:"bytes,6,opt,name=nonHashData" json:"nonHashData,omitempty"`
}
type Transaction struct {
	Type        int32                     `protobuf:"bytes,1,opt,name=type" json:"type"`
	ChaincodeID string                    `protobuf:"bytes,2,opt,name=chaincodeID" json:"chaincodeID"`
	Payload     string                    `protobuf:"bytes,3,opt,name=payload" json:"payload"`
	UUID        string                    `protobuf:"bytes,4,opt,name=uuid" json:"uuid"`
	Timestamp   google_protobuf.Timestamp `protobuf:"bytes,5,opt,name=timestamp" json:"timestamp"`
	Cert        string                    `protobuf:"bytes,6,opt,name=cert" json:"cert"`
	Signature   string                    `protobuf:"bytes,7,opt,name=signature" json:"signature"`
}

// GetBlock ...
func (setup *BaseSetupImpl) GetBlock(num uint64) (Block, error) {
	var block Block
	block.Transactions = make([]Transaction, 0)

	b, err := setup.LedgerClient.QueryBlock(num)
	if err != nil {
		return block, nil
	}

	block.StateHash = base64.StdEncoding.EncodeToString(b.Header.DataHash)
	block.PreviousBlockHash = base64.StdEncoding.EncodeToString(b.Header.PreviousHash)

	for i := 0; i < len(b.Data.Data); i++ {
		// parse block
		envelope, err := utils.ExtractEnvelope(b, i)

		if err != nil {
			return block, err
		}

		payload, err := utils.ExtractPayload(envelope)
		if err != nil {
			return block, err
		}

		channelHeader, err := utils.UnmarshalChannelHeader(payload.Header.ChannelHeader)
		if err != nil {
			return block, err
		}

		block.Version = int(cb.HeaderType(channelHeader.Type))
		switch cb.HeaderType(channelHeader.Type) {
		case cb.HeaderType_MESSAGE:
			break
		case cb.HeaderType_CONFIG:
			configEnvelope := &cb.ConfigEnvelope{}
			if err := proto.Unmarshal(payload.Data, configEnvelope); err != nil {
				return block, err
			}
			break
		case cb.HeaderType_CONFIG_UPDATE:
			configUpdateEnvelope := &cb.ConfigUpdateEnvelope{}
			if err := proto.Unmarshal(payload.Data, configUpdateEnvelope); err != nil {
				return block, err
			}
			break
		case cb.HeaderType_ENDORSER_TRANSACTION:
			tx, err := utils.GetTransaction(payload.Data)
			if err != nil {
				return block, err
			}

			channelHeader := &cb.ChannelHeader{}
			if err := proto.Unmarshal(payload.Header.ChannelHeader, channelHeader); err != nil {
				return block, err
			}

			signatureHeader := &cb.SignatureHeader{}
			if err := proto.Unmarshal(payload.Header.SignatureHeader, signatureHeader); err != nil {
				return block, err
			}

			for _, action := range tx.Actions {
				var transaction Transaction
				transaction.ChaincodeID = setup.ChainCodeID
				transaction.Payload = base64.StdEncoding.EncodeToString(action.Payload)

				transaction.Type = channelHeader.Type
				transaction.UUID = channelHeader.TxId
				transaction.Cert = base64.StdEncoding.EncodeToString(signatureHeader.Creator)
				transaction.Signature = base64.StdEncoding.EncodeToString(envelope.Signature)
				if channelHeader != nil && channelHeader.Timestamp != nil {
					transaction.Timestamp.Seconds = channelHeader.Timestamp.Seconds
					transaction.Timestamp.Nanos = channelHeader.Timestamp.Nanos
				}

				block.Transactions = append(block.Transactions, transaction)
			}
			break
		case cb.HeaderType_ORDERER_TRANSACTION:
			break
		case cb.HeaderType_DELIVER_SEEK_INFO:
			break
		default:
			return block, fmt.Errorf("Unknown message")
		}
	}
	return block, nil
}

//func GetTransaction(ledgerClient *ledger.Client, ccID string,txID fabAPI.TransactionID)(Transaction, error) {
func (setup *BaseSetupImpl)GetTransaction(txID string)(Transaction, error) {

	var transaction Transaction
	// Test Query Transaction -- verify that valid transaction has been processed
	processedTransaction, err := setup.LedgerClient.QueryTransaction(fabAPI.TransactionID(txID))
	//processedTransaction, err := ledgerClient.QueryTransaction(txID, ledger.WithTargetEndpoints(targets...))
	if err != nil {
		fmt.Printf("QueryTransaction return error: %s", err)
	}

	if processedTransaction.TransactionEnvelope == nil {
		fmt.Printf("QueryTransaction failed to return transaction envelope")
	}


	payload, err := utils.ExtractPayload(processedTransaction.TransactionEnvelope)
	if err != nil {
		return transaction, err
	}

	channelHeader, err := utils.UnmarshalChannelHeader(payload.Header.ChannelHeader)
	if err != nil {
		return transaction, err
	}

	switch cb.HeaderType(channelHeader.Type) {
	case cb.HeaderType_MESSAGE:
		break
	case cb.HeaderType_CONFIG:
		configEnvelope := &cb.ConfigEnvelope{}
		if err := proto.Unmarshal(payload.Data, configEnvelope); err != nil {
			return transaction, err
		}
		break
	case cb.HeaderType_CONFIG_UPDATE:
		configUpdateEnvelope := &cb.ConfigUpdateEnvelope{}
		if err := proto.Unmarshal(payload.Data, configUpdateEnvelope); err != nil {
			return transaction, err
		}
		break
	case cb.HeaderType_ENDORSER_TRANSACTION:
		tx, err := utils.GetTransaction(payload.Data)
		if err != nil {
			return transaction, err
		}

		channelHeader := &cb.ChannelHeader{}
		if err := proto.Unmarshal(payload.Header.ChannelHeader, channelHeader); err != nil {
			return transaction, err
		}

		signatureHeader := &cb.SignatureHeader{}
		if err := proto.Unmarshal(payload.Header.SignatureHeader, signatureHeader); err != nil {
			return transaction, err
		}

		for _, action := range tx.Actions {
			ccActionPayload, ccAction, _ := utils.GetPayloads(action)


			ChaincodeProposalPayload, _ := utils.GetChaincodeProposalPayload(ccActionPayload.ChaincodeProposalPayload)

			cis := &peer.ChaincodeInvocationSpec{}
			err = proto.Unmarshal(ChaincodeProposalPayload.Input, cis)

			txRWSet := &rwsetutil.TxRwSet{}
			err = txRWSet.FromProtoBytes(ccAction.Results)

			transaction.ChaincodeID = setup.ChannelID
			transaction.Payload = base64.StdEncoding.EncodeToString(action.Payload)
			transaction.Type = channelHeader.Type
			transaction.UUID = channelHeader.TxId
			transaction.Cert = base64.StdEncoding.EncodeToString(signatureHeader.Creator)
			transaction.Signature = base64.StdEncoding.EncodeToString(processedTransaction.TransactionEnvelope.Signature)
			if channelHeader != nil && channelHeader.Timestamp != nil {
				transaction.Timestamp.Seconds = channelHeader.Timestamp.Seconds
				transaction.Timestamp.Nanos = channelHeader.Timestamp.Nanos
			}
		}
		break
	case cb.HeaderType_ORDERER_TRANSACTION:
		break
	case cb.HeaderType_DELIVER_SEEK_INFO:
		break
	default:
		return transaction, fmt.Errorf("Unknown message")
	}

	return transaction, nil


}
// Invoke ...
func (setup *BaseSetupImpl) Invoke(key string, value string) (fabAPI.TransactionID, error) {
	return SetKeyData(setup.OrgChannelClientContext,setup.ChainCodeID,key,value),nil
}


//ResetKeys resets given set of keys in example cc to given value
func SetKeyData(ctx contextAPI.ChannelProvider, chaincodeID, key string,value string) fabAPI.TransactionID{
	chClient, err := channel.New(ctx)
	if err != nil {print(err)}

	// Synchronous transaction
	respone, e := chClient.Execute(
		channel.Request{
			ChaincodeID: chaincodeID,
			Fcn:         "invoke",
			Args:        CCTxSetArgs(key, value),
		},
		channel.WithRetry(retry.DefaultChannelOpts))

	if e != nil {print(e)}

	return respone.TransactionID
}

// Invoke ...
func (setup *BaseSetupImpl) FindByKeys(fcn string,key string, key2 string) (string, error) {
	return GetValueFromKeys(setup.ChClient,setup.ChainCodeID,fcn,key,key2),nil
}


//ResetKeys resets given set of keys in example cc to given value
func GetValueFromKeys(chClient *channel.Client,ccID,fcn string, key string,key2 string) string{

	const (
		maxRetries = 10
		retrySleep = 500 * time.Millisecond
	)
	for r := 0; r < maxRetries; r++ {
		response, err := chClient.Query(channel.Request{ChaincodeID: ccID, Fcn: "invoke", Args: CCQueryByKeys(fcn,key,key2)},
			channel.WithRetry(retry.DefaultChannelOpts))
		if err == nil {
			actual := string(response.Payload)
			if actual != "" {
				return actual
			}
		}

		time.Sleep(retrySleep)
	}

	return ""
}

// Query ...
func (setup *BaseSetupImpl) Find(fcn string,key string) (string, error){
	return GetValueFromKey(setup.ChClient,setup.ChainCodeID,fcn,key),nil
}

func GetValueFromKey(chClient *channel.Client,ccID, fcn string,key string) string{

	const (
		maxRetries = 10
		retrySleep = 500 * time.Millisecond
	)

	for r := 0; r < maxRetries; r++ {
		response, err := chClient.Query(channel.Request{ChaincodeID: ccID, Fcn: "invoke", Args: CCFindArgs(fcn,key)},
			channel.WithRetry(retry.DefaultChannelOpts))
		if err == nil {
			actual := string(response.Payload)
			if actual != "" {
				return actual
			}
		}

		time.Sleep(retrySleep)
	}

	return ""
}