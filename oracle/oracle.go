package oracle

import (
	"context"
	"fmt"

	"github.com/republicprotocol/republic-go/crypto"
	"github.com/republicprotocol/republic-go/dispatch"
	"github.com/republicprotocol/republic-go/identity"
	"github.com/republicprotocol/republic-go/logger"
	"github.com/republicprotocol/republic-go/swarm"
)

// A Client exposes methods for invoking RPCs on a remote server.
type Client interface {
	// UpdateMidpoint sends a midpointPrice to the target MultiAddress.
	UpdateMidpoint(ctx context.Context, to identity.MultiAddress, midpointPrice MidpointPrice) error

	// MultiAddress returns the multiAddress of the client.
	MultiAddress() identity.MultiAddress
}

type Oracler interface {
	// UpdateMidpoint sends the given midpoint information to α randomly
	// selected nodes.
	UpdateMidpoint(ctx context.Context, midpointPrice MidpointPrice) error
}

type oracler struct {
	client          Client
	key             *crypto.EcdsaKey
	multiAddrStorer swarm.MultiAddressStorer
	α               int
}

// NewOracler returns an object that implements the Oracler interface.
func NewOracler(client Client, key *crypto.EcdsaKey, multiAddrStorer swarm.MultiAddressStorer, α int) Oracler {
	return &oracler{
		client:          client,
		key:             key,
		multiAddrStorer: multiAddrStorer,
		α:               α,
	}
}

// UpdateMidpoint implements the Oracler interface.
func (oracler *oracler) UpdateMidpoint(ctx context.Context, midpointPrice MidpointPrice) error {
	randomMultiAddrs, err := swarm.RandomMultiAddrs(oracler.multiAddrStorer, oracler.client.MultiAddress().Address(), oracler.α)
	if err != nil {
		return err
	}

	// Forward updated midpoint information to α randomly selected nodes.
	var errs = make([]error, len(randomMultiAddrs))
	dispatch.CoForAll(randomMultiAddrs, func(i int) {
		errs[i] = oracler.client.UpdateMidpoint(ctx, randomMultiAddrs[i], midpointPrice)
		if errs[i] != nil {
			logger.Error(fmt.Sprintf("cannot send midpoint price to %v: %v", randomMultiAddrs[i].Address(), errs[i]))
		}
	})

	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

type Server interface {
	// UpdateMidpoint verifies and stores updated midpoint information into a
	// storer and broadcasts this information to the network if it is new
	// information.
	UpdateMidpoint(ctx context.Context, midpointPrice MidpointPrice) error
}

type server struct {
	oracler             Oracler
	oracleAddr          identity.Address
	multiAddrStorer     swarm.MultiAddressStorer
	midpointPriceStorer MidpointPriceStorer
	α                   int
}

// NewServer returns an object that implements the Server interface.
func NewServer(oracler Oracler, oracleAddr identity.Address, multiAddrStorer swarm.MultiAddressStorer, midpointPriceStorer MidpointPriceStorer, α int) Server {
	return &server{
		oracler:             oracler,
		oracleAddr:          oracleAddr,
		multiAddrStorer:     multiAddrStorer,
		midpointPriceStorer: midpointPriceStorer,
		α:                   α,
	}
}

// UpdateMidpoint implements the Server interface.
func (server *server) UpdateMidpoint(ctx context.Context, midpointPrice MidpointPrice) error {
	// Verifies the signature.
	verifier := crypto.NewEcdsaVerifier(server.oracleAddr.String())
	if err := verifier.Verify(midpointPrice.Hash(), midpointPrice.Signature); err != nil {
		return fmt.Errorf("failed to verify midpoint price signature: %v", err)
	}
	oldNonce, err := server.midpointPriceStorer.Nonce()
	if err != nil {
		return err
	}

	// If the midpoint information has been updated, gossip the new information
	// to α random nodes in the network using the Oracler.
	if oldNonce == 0 || midpointPrice.Nonce > oldNonce {
		server.midpointPriceStorer.PutMidpointPrice(midpointPrice)
		return server.oracler.UpdateMidpoint(ctx, midpointPrice)
	}

	return nil
}
