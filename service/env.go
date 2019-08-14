package service

import (
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/util/pathvar"
	"os"
)

func FetchConfigBackend() core.ConfigProvider {
	//sdk net config sdknet_dev.yaml
	path := GetSDKConfigFile()
	configProvider := config.FromFile(pathvar.Subst(path))

	if IsLocal() {
		return func() ([]core.ConfigBackend,error) {
			return appendLocalEntityMappingBackend(configProvider,GetLocalEntityMatcher())
		}
	}
	return configProvider
}
//IsLocal checks os argument and returns true if 'testLocal=true' argument found
func IsLocal() bool {
	args := os.Args[1:]
	for _,arg := range args {
		if arg == "testLocal=true"{
			return true
		}
	}
	return false
}

//appendLocalEntityMappingBackend appends entity matcher backend to given config provider
func appendLocalEntityMappingBackend(configProvider core.ConfigProvider,entityMatcherOverridePath string)([]core.ConfigBackend,error) {
	currentBackends,err := extractBackend(configProvider)
	if err!=nil {
		return nil,err
	}

	//Entity matcher config backend
	configProvider = config.FromFile(pathvar.Subst(entityMatcherOverridePath))
	matcherBackends,err := configProvider()
	if err!=nil{
		return nil,err
	}

	//backends should fal back in this order - matcherBackends,localBackends,currentBackends
	localBackends := append([]core.ConfigBackend{},matcherBackends...)
	localBackends = append(localBackends,currentBackends...)

	return localBackends,nil
}

func extractBackend(configProvider core.ConfigProvider) ([]core.ConfigBackend,error) {
		if configProvider == nil {
			return []core.ConfigBackend{},nil
		}
		return configProvider()
}