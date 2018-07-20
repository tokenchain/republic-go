package grpc

import (
	"fmt"

	"github.com/republicprotocol/republic-go/identity"
	"github.com/republicprotocol/republic-go/logger"
	"github.com/republicprotocol/republic-go/swarm"
	"golang.org/x/net/context"
)

type swarmClient struct {
	store swarm.MultiAddressStorer
}

// NewSwarmClient returns an implementation of the swarm.Client interface that
// uses gRPC and a recycled connection pool.
func NewSwarmClient(store swarm.MultiAddressStorer) swarm.Client {
	return &swarmClient{
		store: store,
	}
}

// Ping implements the swarm.Client interface.
func (client *swarmClient) Ping(ctx context.Context, to identity.MultiAddress, multiAddr identity.MultiAddress, nonce uint64) error {
	conn, err := Dial(ctx, to)
	if err != nil {
		logger.Network(logger.LevelError, fmt.Sprintf("cannot dial %v: %v", to, err))
		return fmt.Errorf("cannot dial %v: %v", to, err)
	}
	defer conn.Close()

	request := &PingRequest{
		Signature:         multiAddr.Signature,
		MultiAddress:      multiAddr.String(),
		MultiAddressNonce: nonce,
	}

	return Backoff(ctx, func() error {
		_, err = NewSwarmServiceClient(conn).Ping(ctx, request)
		return err
	})
}

func (client *swarmClient) Pong(ctx context.Context, to identity.MultiAddress) error {
	conn, err := Dial(ctx, to)
	if err != nil {
		logger.Network(logger.LevelError, fmt.Sprintf("cannot dial %v: %v", to, err))
		return fmt.Errorf("cannot dial %v: %v", to, err)
	}
	defer conn.Close()

	multiAddr, nonce, err := client.store.Self()
	if err != nil {
		logger.Network(logger.LevelError, fmt.Sprintf("cannot get self details: %v", err))
		return fmt.Errorf("cannot get self details: %v", err)
	}

	request := &PongRequest{
		Signature:         multiAddr.Signature,
		MultiAddress:      multiAddr.String(),
		MultiAddressNonce: nonce,
	}

	return Backoff(ctx, func() error {
		_, err = NewSwarmServiceClient(conn).Pong(ctx, request)
		return err
	})
}

// Query implements the swarm.Client interface.
func (client *swarmClient) Query(ctx context.Context, to identity.MultiAddress, query identity.Address, querySignature [65]byte) (identity.MultiAddresses, error) {
	conn, err := Dial(ctx, to)
	if err != nil {
		logger.Network(logger.LevelError, fmt.Sprintf("cannot dial %v: %v", to, err))
		return identity.MultiAddresses{}, fmt.Errorf("cannot dial %v: %v", to, err)
	}
	defer conn.Close()

	request := &QueryRequest{
		Signature: querySignature[:],
		Address:   query.String(),
	}

	var response *QueryResponse
	if err := Backoff(ctx, func() error {
		response, err = NewSwarmServiceClient(conn).Query(ctx, request)
		return err
	}); err != nil {
		return identity.MultiAddresses{}, err
	}

	multiAddrs := identity.MultiAddresses{}
	for _, multiAddrStr := range response.MultiAddresses {
		multiAddr, err := identity.NewMultiAddressFromString(multiAddrStr)
		if err != nil {
			logger.Network(logger.LevelWarn, fmt.Sprintf("cannot parse %v: %v", multiAddrStr, err))
			continue
		}
		multiAddr.Signature = multiAddr.Hash()
		multiAddrs = append(multiAddrs, multiAddr)
	}
	return multiAddrs, nil
}

// MultiAddress implements the swarm.Client interface.
func (client *swarmClient) MultiAddress() identity.MultiAddress {
	multiAddr, _, err := client.store.Self()
	if err != nil {
		logger.Network(logger.LevelError, fmt.Sprintf("cannot retrieve own multiaddress: %v", err))
		return identity.MultiAddress{}
	}
	return multiAddr
}

// SwarmService is a Service that implements the gRPC SwarmService defined in
// protobuf. It delegates responsibility for handling the Ping and Query RPCs
// to a swarm.Server.
type SwarmService struct {
	server swarm.Server
}

// NewSwarmService returns a SwarmService that uses the swarm.Server as a
// delegate.
func NewSwarmService(server swarm.Server) SwarmService {
	return SwarmService{
		server: server,
	}
}

// Register implements the Service interface.
func (service *SwarmService) Register(server *Server) {
	RegisterSwarmServiceServer(server.Server, service)
}

// Ping is an RPC used to notify a SwarmService about the existence of a
// client. In the PingRequest, the client sends a signed identity.MultiAddress
// and the SwarmService delegates the responsibility of handling this signed
// identity.MultiAddress to its swarm.Server. If its swarm.Server accepts the
// signed identity.MultiAddress of the client it will return its own signed
// identity.MultiAddress in a PingResponse.
func (service *SwarmService) Ping(ctx context.Context, request *PingRequest) (*PingResponse, error) {
	from, err := identity.NewMultiAddressFromString(request.GetMultiAddress())
	if err != nil {
		logger.Network(logger.LevelError, fmt.Sprintf("cannot unmarshal multiaddress: %v", err))
		return nil, fmt.Errorf("cannot unmarshal multiaddress: %v", err)
	}

	from.Signature = request.GetSignature()
	nonce := request.GetMultiAddressNonce()

	err = service.server.Ping(ctx, from, nonce)
	if err != nil {
		logger.Network(logger.LevelInfo, fmt.Sprintf("cannot update store with: %v", err))
		return &PingResponse{}, fmt.Errorf("cannot update store: %v", err)
	}
	return &PingResponse{}, nil
}

// Pong is an RPC used to notify a SwarmService about the existence of a
// client. In the PingRequest, the client sends a signed identity.MultiAddress
// and the SwarmService delegates the responsibility of handling this signed
// identity.MultiAddress to its swarm.Server. If its swarm.Server accepts the
// signed identity.MultiAddress of the client it will return its own signed
// identity.MultiAddress in a PingResponse.
func (service *SwarmService) Pong(ctx context.Context, request *PongRequest) (*PongResponse, error) {
	from, err := identity.NewMultiAddressFromString(request.GetMultiAddress())
	if err != nil {
		logger.Network(logger.LevelError, fmt.Sprintf("cannot unmarshal multiaddress: %v", err))
		return nil, fmt.Errorf("cannot unmarshal multiaddress: %v", err)
	}

	from.Signature = request.GetSignature()
	nonce := request.GetMultiAddressNonce()

	err = service.server.Pong(ctx, from, nonce)
	if err != nil {
		logger.Network(logger.LevelInfo, fmt.Sprintf("cannot update storer with %v: %v", request.GetMultiAddress(), err))
		return &PongResponse{}, fmt.Errorf("cannot update storer: %v", err)
	}
	return &PongResponse{}, nil
}

// Query is an RPC used to find identity.MultiAddresses. In the QueryRequest,
// the client sends an identity.Address and the SwarmService will stream
// identity.MultiAddresses to the client. The SwarmService delegates
// responsibility to its swarm.Server to return identity.MultiAddresses that
// are close to the queried identity.Address.
func (service *SwarmService) Query(ctx context.Context, request *QueryRequest) (*QueryResponse, error) {
	query := identity.Address(request.GetAddress())
	querySig := [65]byte{}
	copy(querySig[:], request.GetSignature())

	multiAddrs, err := service.server.Query(ctx, query, querySig)
	if err != nil {
		return nil, err
	}

	multiAddrsStr := make([]string, len(multiAddrs))
	for i, multiAddr := range multiAddrs {
		multiAddrsStr[i] = multiAddr.String()
	}

	return &QueryResponse{
		Signature:      []byte{},
		MultiAddresses: multiAddrsStr,
	}, nil
}
