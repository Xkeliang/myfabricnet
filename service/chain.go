package service

import (
	"encoding/json"
	"fmt"
	"github.com/fjl/go-couchdb"
	"github.com/gocraft/web"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric/protos/ledger/queryresult"
	"io/ioutil"
	"net/http"
	"strconv"
)
type Chain struct {
	*Blockchain
}

// History worldState history
type History struct {
	*queryresult.KeyModification `json:"history"`
	Value                        Car `json:"value"`
	BlockNum                     uint64 `json:"blockNum"`
}

// QueryResult chaincode query result
type QueryResult struct {
	Current struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"current"`
	Historys []History `json:"historys"`
}

func (c *Chain)chain(rw web.ResponseWriter,rq *web.Request)  {
	encoder := json.NewEncoder(rw)

	chain,err := c.BSI.GetBlockchainInfo()
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Errorf("query chaininfo failed: %s", err)
		return
	}
	rw.WriteHeader(http.StatusOK)
	err = encoder.Encode(Status{Code: http.StatusOK, Message: chain})
	if err != nil{
		logger.Errorf("invoke return failed: %s", err)
	}
}
// block
func (c *Chain) block(rw web.ResponseWriter, req *web.Request) {
	encoder := json.NewEncoder(rw)

	blockNumber, err := strconv.Atoi(req.PathParams["num"])
	if err != nil {
		// Failure
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
	block, err := c.BSI.GetBlock(uint64(blockNumber))
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Errorf("QueryBlock error:%v", err)
		return
	}

	rw.WriteHeader(http.StatusOK)
	err = encoder.Encode(Status{Code: http.StatusOK, Message: block})
	if err != nil{
		logger.Errorf("invoke return failed: %s", err)
	}
}

// tx ...
func (c *Chain) tx(rw web.ResponseWriter, req *web.Request) {
	encoder := json.NewEncoder(rw)

	txid := req.PathParams["txid"]
	tx, err := c.BSI.GetTransaction(txid)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Errorf("QueryBlock error:%v", err)
		return
	}

	rw.WriteHeader(http.StatusOK)
	err = encoder.Encode(Status{Code: http.StatusOK, Message: tx})
	if err != nil{
		logger.Errorf("invoke return failed: %s", err)
	}
}

// queryKV
func (c *Chain) queryKV(rw web.ResponseWriter, req *web.Request) {
	encoder := json.NewEncoder(rw)

	key := req.PathParams["key"]
	keyPre := req.PathParams["keyPre"]

	result, err := getKV(c.BSI, []string{key}, keyPre) //setup.Query("query", []string{key})
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Errorf("invoke chaincode failed: %s", err)
		return
	}

	rw.WriteHeader(http.StatusOK)
	err = encoder.Encode(Status{Code: http.StatusOK, Message: result})
	if err != nil{
		logger.Errorf("invoke return failed: %s", err)
	}
}

// invoke ...
func (c *Chain) invoke(rw web.ResponseWriter, req *web.Request) {
	encoder := json.NewEncoder(rw)

	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		logger.Error("Internal JSON error when reading request body.")
		return
	}

	// Incoming request body may not be empty, client must supply request payload
	if string(reqBody) == "" {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Error("Client must supply a payload for order requests.")
		return
	}
	fmt.Printf("Req body: %s", string(reqBody))

	var args Car=Car{}
	err = json.Unmarshal(reqBody, &args)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Errorf("Error unmarshalling request payload: %s", err)
		return
	}

	txid, err := c.BSI.Invoke(args.ID, string(reqBody))
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Errorf("invoke chaincode failed: %s", err)
		return
	}


	if rev,err :=CouchDb.Rev(args.ID); err ==nil{
		_,_=CouchDb.Put(args.ID,args,rev)
	}else if couchdb.NotFound(err) {
		_,_=CouchDb.Put(args.ID,args,"")
	}else {
		fmt.Println("err:",err)
	}
	rw.WriteHeader(http.StatusOK)
	err = encoder.Encode(Status{Code: http.StatusOK, Message: txid})
	if err != nil{
		logger.Errorf("invoke return failed: %s", err)
	}
}

// query ...
func (c *Chain) query(rw web.ResponseWriter, req *web.Request) {
	encoder := json.NewEncoder(rw)

	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		logger.Error("Internal JSON error when reading request body.")
		return
	}

	// Incoming request body may not be empty, client must supply request payload
	if string(reqBody) == "" {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Error("Client must supply a payload for order requests.")
		return
	}
	fmt.Printf("Req body: %s \n", string(reqBody))

	var args Car = Car{}
	err = json.Unmarshal(reqBody, &args)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Errorf("Error unmarshalling request payload: %s", err)
		return
	}
	//couchDb query
	var carMessage string
	var carGet *Car = &Car{}
	err = CouchDb.Get(args.ID,carGet,nil)

	if err==nil {
		fmt.Println("get from couchDbCache")
		rw.WriteHeader(http.StatusOK)
		err = encoder.Encode(Status{Code: http.StatusOK, Message: carGet})
		if err != nil{
			logger.Errorf("query return failed: %s", err)
		}
	}else{
		carMessage, err = c.BSI.Find("query",args.ID)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			logger.Errorf("invoke chaincode failed: %s", err)
			return
		}
		//解析carMessage
		var carPut  Car = Car{}
		carPut = Message2Car(carMessage)
		// write  to couchDbCache
		_,err =CouchDb.Put(args.ID,carPut,"")
		if err!=nil{
			fmt.Println("put result failed",carMessage+".err:",err)
		}
		rw.WriteHeader(http.StatusOK)
		err = encoder.Encode(Status{Code: http.StatusOK, Message: carMessage})
		if err != nil{
			logger.Errorf("query return failed: %s", err)
		}
	}
}

func Message2Car(str string) (car Car) {

	car = Car{}
	err :=json.Unmarshal([]byte(str),&car)
	if err!=nil{
		return car
	}
	return car
}

// queryWithBlock ...
func (c *Chain) queryWithBlock(rw web.ResponseWriter, req *web.Request) {
	encoder := json.NewEncoder(rw)

	key := req.PathParams["key"]
	result, err := c.BSI.Find("history",key)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Errorf("query chaincode failed: %s", err)
		return
	}

	query := QueryResult{}

	err = json.Unmarshal([]byte(result), &query)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Errorf("query chaincode failed: %s", err)
		return
	}

	for k, v := range query.Historys {
		var carHistory Car
		err = json.Unmarshal(v.GetValue(),&carHistory)
		if err!=nil{
			logger.Errorf("Unmarshal failed: %s", err)
		}
		query.Historys[k].Value = carHistory

		// 在Channel接口中添加QueryBlockByTxID方法并实现
		block, err := c.BSI.LedgerClient.QueryBlockByTxID(fab.TransactionID(v.GetTxId()))
		if err != nil {
			logger.Errorf("QueryBlockByTxID error:%v", err)
			continue
		}

		query.Historys[k].BlockNum = block.Header.Number
	}

	rw.WriteHeader(http.StatusOK)
	err = encoder.Encode(Status{Code: http.StatusOK, Message: query})
	if err != nil{
		logger.Errorf("query return failed: %s", err)
	}
}
// query ...GroupBy car color
func (c *Chain) queryGroup(rw web.ResponseWriter, req *web.Request) {
	encoder := json.NewEncoder(rw)

	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		logger.Error("Internal JSON error when reading request body.")
		return
	}

	// Incoming request body may not be empty, client must supply request payload
	if string(reqBody) == "" {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Error("Client must supply a payload for order requests.")
		return
	}
	fmt.Printf("queryGroup Req body: %s", string(reqBody))

	var args =Car{}
	err = json.Unmarshal(reqBody, &args)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Errorf("Error unmarshalling request payload: %s", err)
		return
	}

	result, err := c.BSI.Find("queryGroup",args.Color)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Errorf("invoke chaincode failed: %s", err)
		return
	}

	rw.WriteHeader(http.StatusOK)
	err = encoder.Encode(Status{Code: http.StatusOK, Message: result})
	if err != nil{
		logger.Errorf("query return failed: %s", err)
	}
}


// query ...queryRange car key
func (c *Chain) queryRange(rw web.ResponseWriter, req *web.Request) {
	encoder := json.NewEncoder(rw)

	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		logger.Error("Internal JSON error when reading request body.")
		return
	}

	// Incoming request body may not be empty, client must supply request payload
	if string(reqBody) == "" {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Error("Client must supply a payload for order requests.")
		return
	}
	fmt.Printf("Req body: %s", string(reqBody))

	var args struct{
		Args []string `json:"args"`
	}
	err = json.Unmarshal(reqBody, &args)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Errorf("Error unmarshalling request payload: %s", err)
		return
	}

	result, err := c.BSI.FindByKeys("queryRange",args.Args[0],args.Args[1])
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Errorf("invoke chaincode failed: %s", err)
		return
	}

	rw.WriteHeader(http.StatusOK)
	err = encoder.Encode(Status{Code: http.StatusOK, Message: result})
	if err != nil{
		logger.Errorf("query return failed: %s", err)
	}
}

// query ...queryLike  key
func (c *Chain) queryLike(rw web.ResponseWriter, req *web.Request) {
	encoder := json.NewEncoder(rw)

	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		logger.Error("Internal JSON error when reading request body.")
		return
	}

	// Incoming request body may not be empty, client must supply request payload
	if string(reqBody) == "" {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Error("Client must supply a payload for order requests.")
		return
	}
	fmt.Printf("Req body: %s", string(reqBody))

	var args =Car{}
	err = json.Unmarshal(reqBody, &args)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Errorf("Error unmarshalling request payload: %s", err)
		return
	}

	result, err := c.BSI.Find("queryLike",args.ID)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		logger.Errorf("invoke chaincode failed: %s", err)
		return
	}

	rw.WriteHeader(http.StatusOK)
	err = encoder.Encode(Status{Code: http.StatusOK, Message: result})
	if err != nil{
		logger.Errorf("query return failed: %s", err)
	}
}