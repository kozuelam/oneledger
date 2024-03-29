package broadcast

import (
	"errors"
	"github.com/Oneledger/protocol/data/fees"

	"github.com/Oneledger/protocol/data/balance"

	"github.com/Oneledger/protocol/action"
	"github.com/Oneledger/protocol/client"
	"github.com/Oneledger/protocol/log"
	"github.com/Oneledger/protocol/rpc"
	"github.com/Oneledger/protocol/serialize"
)

type Service struct {
	logger     *log.Logger
	router     action.Router
	currencies *balance.CurrencySet
	feeOpt     *fees.FeeOption
	ext        client.ExtServiceContext
}

func NewService(ctx client.ExtServiceContext, router action.Router, currencies *balance.CurrencySet, feeOpt *fees.FeeOption, logger *log.Logger) *Service {
	return &Service{
		ext:        ctx,
		router:     router,
		currencies: currencies,
		feeOpt:     feeOpt,
		logger:     logger,
	}
}

// Name returns the name of this service. The RPC method will be prefixed with this service name plus a . (e.g. "broadcast.")
func Name() string {
	return "broadcast"
}
func (svc *Service) validateAndSignTx(req client.BroadcastRequest) ([]byte, error) {
	var tx action.RawTx
	err := serialize.GetSerializer(serialize.NETWORK).Deserialize(req.RawTx, &tx)
	if err != nil {
		err = rpc.InvalidRequestError("invalid rawTx given")
		return nil, err
	}

	sigs := []action.Signature{{Signer: req.PublicKey, Signed: req.Signature}}
	signedTx := action.SignedTx{
		RawTx:      tx,
		Signatures: sigs,
	}

	handler := svc.router.Handler(tx.Type)
	ctx := action.NewContext(svc.router, nil, nil, nil, nil, svc.currencies, svc.feeOpt, nil, nil, nil, svc.logger)
	_, err = handler.Validate(ctx, signedTx)
	if err != nil {
		err = rpc.InvalidRequestError(err.Error())
		return nil, err
	}

	return signedTx.SignedBytes(), nil
}

func (svc *Service) broadcast(method client.BroadcastMode, req client.BroadcastRequest) (client.BroadcastReply, error) {
	makeErr := func(err error) error { return rpc.InternalError(err.Error()) }

	rawSignedTx, err := svc.validateAndSignTx(req)
	if err != nil {
		return client.BroadcastReply{}, err
	}

	reply := new(client.BroadcastReply)

	switch method {
	case client.BROADCASTSYNC:
		result, err := svc.ext.BroadcastTxSync(rawSignedTx)
		if err != nil {
			return client.BroadcastReply{}, makeErr(err)
		}
		reply.FromResultBroadcastTx(result)
		return *reply, nil
	case client.BROADCASTASYNC:
		result, err := svc.ext.BroadcastTxAsync(rawSignedTx)
		if err != nil {
			return client.BroadcastReply{}, makeErr(err)
		}
		reply.FromResultBroadcastTx(result)
		return *reply, nil
	case client.BROADCASTCOMMIT:
		result, err := svc.ext.BroadcastTxCommit(rawSignedTx)
		if err != nil {
			return client.BroadcastReply{}, makeErr(err)
		}
		reply.FromResultBroadcastTxCommit(result)
		return *reply, nil
	default:
		return client.BroadcastReply{}, makeErr(errors.New("invalid method string"))
	}
}

// TxAsync returns as soon as the finishes. Returns with a hash
func (svc *Service) TxAsync(req client.BroadcastRequest, reply *client.BroadcastReply) error {
	out, err := svc.broadcast(client.BROADCASTASYNC, req)
	if err != nil {
		return err
	}
	*reply = out
	return nil
}

// TxSync returns when the transaction has been placed inside the mempool
func (svc *Service) TxSync(req client.BroadcastRequest, reply *client.BroadcastReply) error {
	out, err := svc.broadcast(client.BROADCASTSYNC, req)
	if err != nil {
		return err
	}
	*reply = out
	return nil
}

// TxCommit returns when the transaction has been committed to a block.
func (svc *Service) TxCommit(req client.BroadcastRequest, reply *client.BroadcastReply) error {
	out, err := svc.broadcast(client.BROADCASTCOMMIT, req)
	if err != nil {
		return err
	}
	*reply = out
	return nil
}
