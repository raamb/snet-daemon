package cmd

import (
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/singnet/snet-daemon/configuration_service"
	"github.com/singnet/snet-daemon/pricing"
	"github.com/singnet/snet-daemon/metrics"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"

	"github.com/singnet/snet-daemon/blockchain"
	"github.com/singnet/snet-daemon/config"
	"github.com/singnet/snet-daemon/escrow"
	"github.com/singnet/snet-daemon/etcddb"
	"github.com/singnet/snet-daemon/handler"
)

type Components struct {
	serviceMetadata            *blockchain.ServiceMetadata
	blockchain                 *blockchain.Processor
	etcdClient                 *etcddb.EtcdClient
	etcdServer                 *etcddb.EtcdServer
	atomicStorage              escrow.AtomicStorage
	paymentChannelService      escrow.PaymentChannelService
	escrowPaymentHandler       handler.PaymentHandler
	grpcInterceptor            grpc.StreamServerInterceptor
	paymentChannelStateService *escrow.PaymentChannelStateService
	etcdLockerStorage          *escrow.PrefixedAtomicStorage
	providerControlService     *escrow.ProviderControlService
	daemonHeartbeat            *metrics.DaemonHeartbeat
	paymentStorage             *escrow.PaymentStorage
	priceStrategy              *pricing.PricingStrategy
	configurationService       *configuration_service.ConfigurationService
	configurationBroadcaster   *configuration_service.MessageBroadcaster
	organizationMetaData       *blockchain.OrganizationMetaData
	freeCallPaymentHandler      handler.PaymentHandler
}

func InitComponents(cmd *cobra.Command) (components *Components) {
	components = &Components{}
	defer func() {
		err := recover()
		if err != nil {
			components.Close()
			components = nil
			panic("re-panic after components cleanup")
		}
	}()

	loadConfigFileFromCommandLine(cmd.Flags().Lookup("config"))

	return
}

func loadConfigFileFromCommandLine(configFlag *pflag.Flag) {
	var configFile = configFlag.Value.String()

	// if file is not specified by user then configFile contains default name
	if configFlag.Changed || isFileExist(configFile) {
		err := config.LoadConfig(configFile)
		if err != nil {
			log.WithError(err).WithField("configFile", configFile).Panic("Error reading configuration file")
		}
		log.WithField("configFile", configFile).Info("Using configuration file")
	} else {
		log.Info("Configuration file is not set, using default configuration")
	}

}

func isFileExist(fileName string) bool {
	_, err := os.Stat(fileName)
	return !os.IsNotExist(err)
}

func (components *Components) Close() {
	if components.etcdClient != nil {
		components.etcdClient.Close()
	}
	if components.etcdServer != nil {
		components.etcdServer.Close()
	}
	if components.blockchain != nil {
		components.blockchain.Close()
	}
}

func (components *Components) Blockchain() *blockchain.Processor {
	if components.blockchain != nil {
		return components.blockchain
	}

	processor, err := blockchain.NewProcessor(components.ServiceMetaData())
	if err != nil {
		log.WithError(err).Panic("unable to initialize blockchain processor")
	}

	components.blockchain = &processor
	return components.blockchain
}

func (components *Components) ServiceMetaData() *blockchain.ServiceMetadata {
	if components.serviceMetadata != nil {
		return components.serviceMetadata
	}
	components.serviceMetadata = blockchain.ServiceMetaData()
	return components.serviceMetadata
}

func (components *Components) OrganizationMetaData() *blockchain.OrganizationMetaData {
	if components.organizationMetaData != nil {
		return components.organizationMetaData
	}
	components.organizationMetaData = blockchain.GetOrganizationMetaData()
	return components.organizationMetaData
}


func (components *Components) EtcdServer() *etcddb.EtcdServer {
	if components.etcdServer != nil {
		return components.etcdServer
	}

	enabled, err := etcddb.IsEtcdServerEnabled()
	if err != nil {
		log.WithError(err).Panic("error during etcd config parsing")
	}
	if !enabled {
		return nil
	}

	server, err := etcddb.GetEtcdServer()
	if err != nil {
		log.WithError(err).Panic("error during etcd config parsing")
	}

	err = server.Start()
	if err != nil {
		log.WithError(err).Panic("error during etcd server starting")
	}

	components.etcdServer = server
	return server
}

func (components *Components) EtcdClient() *etcddb.EtcdClient {
	if components.etcdClient != nil {
		return components.etcdClient
	}

	client, err := etcddb.NewEtcdClient(components.OrganizationMetaData())
	if err != nil {
		log.WithError(err).Panic("unable to create etcd client")
	}

	components.etcdClient = client
	return components.etcdClient
}

func (components *Components) LockerStorage() *escrow.PrefixedAtomicStorage {
	if components.etcdLockerStorage != nil {
		return components.etcdLockerStorage
	}
	components.etcdLockerStorage = escrow.NewLockerStorage(components.AtomicStorage(),components.ServiceMetaData())
	return components.etcdLockerStorage
}

func (components *Components) AtomicStorage() escrow.AtomicStorage {
	if components.atomicStorage != nil {
		return components.atomicStorage
	}

	if config.GetString(config.PaymentChannelStorageTypeKey) == "etcd" {
		components.atomicStorage = components.EtcdClient()
	} else {
		components.atomicStorage = escrow.NewMemStorage()
	}

	return components.atomicStorage
}

func (components *Components) PaymentStorage() *escrow.PaymentStorage {
	if components.paymentStorage != nil {
		return components.paymentStorage
	}

	components.paymentStorage = escrow.NewPaymentStorage(components.AtomicStorage())

	return components.paymentStorage
}

func (components *Components) PaymentChannelService() escrow.PaymentChannelService {
	if components.paymentChannelService != nil {
		return components.paymentChannelService
	}

	components.paymentChannelService = escrow.NewPaymentChannelService(
		escrow.NewPaymentChannelStorage(components.AtomicStorage(),components.ServiceMetaData()),
		components.PaymentStorage(),
		escrow.NewBlockchainChannelReader(components.Blockchain(), config.Vip(),components.OrganizationMetaData()),
		escrow.NewEtcdLocker(components.AtomicStorage(),components.ServiceMetaData()),
		escrow.NewChannelPaymentValidator(components.Blockchain(), config.Vip(), components.OrganizationMetaData()), func() ([32]byte, error) {
			s := components.OrganizationMetaData().GetGroupId()
			return s, nil
		},
	)

	return components.paymentChannelService
}

func (components *Components) EscrowPaymentHandler() handler.PaymentHandler {
	if components.escrowPaymentHandler != nil {
		return components.escrowPaymentHandler
	}

	components.escrowPaymentHandler = escrow.NewPaymentHandler(
		components.PaymentChannelService(),
		components.Blockchain(),
		escrow.NewIncomeValidator(components.PricingStrategy()),
	)

	return components.escrowPaymentHandler
}

func (components *Components) FreeCallPaymentHandler() handler.PaymentHandler {
	if components.freeCallPaymentHandler != nil {
		return components.freeCallPaymentHandler
	}

	components.freeCallPaymentHandler = escrow.FreeCallPaymentHandler(
		components.Blockchain(),components.OrganizationMetaData())

	return components.freeCallPaymentHandler
}

//Add a chain of interceptors
func (components *Components) GrpcInterceptor() grpc.StreamServerInterceptor {
	if components.grpcInterceptor != nil {
		return components.grpcInterceptor
	}
    //Metering is now mandatory in Daemon
	metrics.SetDaemonGrpId(components.OrganizationMetaData().GetGroupIdString())
	if components.Blockchain().Enabled() {

		components.grpcInterceptor = grpc_middleware.ChainStreamServer(
			handler.GrpcMonitoringInterceptor(), handler.GrpcRateLimitInterceptor(components.ChannelBroadcast()),
			components.GrpcPaymentValidationInterceptor())
	} else {
		components.grpcInterceptor = grpc_middleware.ChainStreamServer(handler.GrpcRateLimitInterceptor(components.ChannelBroadcast()),
			components.GrpcPaymentValidationInterceptor())
	}
	return components.grpcInterceptor
}

func (components *Components) GrpcPaymentValidationInterceptor() grpc.StreamServerInterceptor {
	if !components.Blockchain().Enabled() {
		log.Info("Blockchain is disabled: no payment validation")
		return handler.NoOpInterceptor
	} else {
		log.Info("Blockchain is enabled: instantiate payment validation interceptor")
		return handler.GrpcPaymentValidationInterceptor(components.EscrowPaymentHandler(),components.FreeCallPaymentHandler())
	}
}

func (components *Components) PaymentChannelStateService() (service escrow.PaymentChannelStateServiceServer) {
	if !config.GetBool(config.BlockchainEnabledKey){
		return &escrow.BlockChainDisabledStateService{}
	}

	if components.paymentChannelStateService != nil {
		return components.paymentChannelStateService
	}

	components.paymentChannelStateService = escrow.NewPaymentChannelStateService(
		components.PaymentChannelService(),
		components.PaymentStorage(),
		components.ServiceMetaData())

	return components.paymentChannelStateService
}

//NewProviderControlService

func (components *Components) ProviderControlService() (service escrow.ProviderControlServiceServer) {

	if !config.GetBool(config.BlockchainEnabledKey){
		return &escrow.BlockChainDisabledProviderControlService{}
	}
	if components.providerControlService != nil {
		return components.providerControlService
	}

	components.providerControlService = escrow.NewProviderControlService(components.PaymentChannelService(),
		components.ServiceMetaData(),components.OrganizationMetaData())
	return components.providerControlService
}

func (components *Components) DaemonHeartBeat() (service *metrics.DaemonHeartbeat) {
	if components.daemonHeartbeat != nil {
		return components.daemonHeartbeat
	}
	metrics.SetDaemonGrpId(components.OrganizationMetaData().GetGroupIdString())
	components.daemonHeartbeat = &metrics.DaemonHeartbeat{DaemonID: metrics.GetDaemonID()}
	return components.daemonHeartbeat
}



func (components *Components) PricingStrategy() *pricing.PricingStrategy {
	if components.priceStrategy != nil {
		return components.priceStrategy
	}

	components.priceStrategy,_ = pricing.InitPricingStrategy(components.ServiceMetaData())

	return components.priceStrategy
}


func (components *Components) ChannelBroadcast() *configuration_service.MessageBroadcaster {
	if components.configurationBroadcaster != nil {
		return components.configurationBroadcaster
	}

	components.configurationBroadcaster = configuration_service.NewChannelBroadcaster()

	return components.configurationBroadcaster
}

func (components *Components) ConfigurationService() *configuration_service.ConfigurationService {
	if components.configurationService != nil {
		return components.configurationService
	}

	components.configurationService = configuration_service.NewConfigurationService(components.ChannelBroadcast())

	return components.configurationService
}
