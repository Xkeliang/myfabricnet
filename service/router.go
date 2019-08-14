package service

import (
	"encoding/json"
	"fmt"
	"github.com/gocraft/web"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/logging"
	"net/http"
)

var (
	DefSetup *BaseSetupImpl
	logger   *logging.Logger
)

type Blockchain struct {
	BSI *BaseSetupImpl
}
//value
type Car struct {
	Color      string `json:"Color"`
	ID         string `json:"ID"`  // key
	Price      string `json:"Price"`
	LaunchDate string `json:"LaunchDate"`
}
// Status REST response
type Status struct {
	Code    int         `json:"code"`
	Message interface{} `json:"message"`
}

func BuildRouter()*web.Router  {

	router := web.New(Blockchain{})
	router.Middleware((*Blockchain).setResponseTypeBSI)
	router.NotFound((*Blockchain).notFound)

	chainRouter := router.Subrouter(Chain{}, "/chain")
	chainRouter.Get("", (*Chain).chain)
	chainRouter.Get("/blocks/:num", (*Chain).block)
	chainRouter.Get("/transactions/:txid", (*Chain).tx)
	chainRouter.Get("/queryWithBlock/:key", (*Chain).queryWithBlock)
	//chainRouter.Get("/queryKV/:keyPre/:key", (*Chain).queryKV)

	chainRouter.Post("/invoke", (*Chain).invoke)
	chainRouter.Post("/query", (*Chain).query)
	chainRouter.Post("/queryGroup", (*Chain).queryGroup)
	chainRouter.Post("/queryRange", (*Chain).queryRange)
	chainRouter.Post("/queryLike", (*Chain).queryLike)

	return router
}
// setResponseType is a middleware function that sets the appropriate response
// headers. Currently, it is setting the "Content-Type" to "application/json" as
// well as the necessary headers in order to enable CORS for Swagger usage.
func (b *Blockchain) setResponseTypeBSI(rw web.ResponseWriter, req *web.Request, next web.NextMiddlewareFunc) {
	rw.Header().Set("Content-Type", "application/json")

	// Enable CORS
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	rw.Header().Set("Access-Control-Allow-Headers", "accept, content-type")

	fmt.Println("set user BSI")
	b.BSI = DefSetup
	next(rw, req)
}


// NotFound returns a custom landing page when a given hyperledger end point
// had not been defined.
func (b *Blockchain) notFound(rw web.ResponseWriter, req *web.Request){

	rw.WriteHeader(http.StatusNotFound)
	err := json.NewEncoder(rw).Encode(Status{Code: http.StatusNotFound, Message: "Insurance endpoint not found."})

	if err != nil {
		panic(fmt.Errorf("error find when do notFound : %s", err))
	}

}
