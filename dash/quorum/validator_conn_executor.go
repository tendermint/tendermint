package quorum

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/dash/quorum/selectpeers"
	"github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/internal/p2p"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/types"
)

const (
	// validatorConnExecutorName contains name that is used to represent the ValidatorConnExecutor in BaseService and logs
	validatorConnExecutorName = "ValidatorConnExecutor"
	// defaultTimeout is timeout that is applied to setup / cleanup code
	defaultTimeout = 1 * time.Second
	// defaultEventBusCapacity determines how many events can wait in the event bus for processing. 10 looks very safe.
	defaultEventBusCapacity = 10
)

type optionFunc func(vc *ValidatorConnExecutor) error

// ValidatorConnExecutor retrieves validator update events and establishes new validator connections
// within the ValidatorSet.
// If it's already connected to a member of current validator set, it will keep that connection.
// Otherwise, it will randomly select some members of the active validator set and connect to them, to ensure
// it has connectivity with at least `NumConnections` members of the active validator set.
//
// Note that we mark peers that are members of active validator set as Persistent, so p2p subsystem
// will retry the connection if it fails.
type ValidatorConnExecutor struct {
	service.BaseService
	proTxHash    types.ProTxHash
	eventBus     *types.EventBus
	dialer       p2p.DashDialer
	subscription types.Subscription

	// validatorSetMembers contains validators active in the current Validator Set, indexed by node ID
	validatorSetMembers validatorMap
	// connectedValidators contains validators we should be connected to, indexed by node ID
	connectedValidators validatorMap
	// quorumHash contains current quorum hash
	quorumHash tmbytes.HexBytes
	// nodeIDResolvers can be used to determine a node ID for a validator
	nodeIDResolvers []p2p.NodeIDResolver
	// mux is a mutex to ensure only one goroutine is processing connections
	mux sync.Mutex

	// *** configuration *** //

	// EventBusCapacity sets event bus buffer capacity, defaults to 10
	EventBusCapacity int
}

var (
	errPeerNotFound = fmt.Errorf("cannot stop peer: not found")
)

// NewValidatorConnExecutor creates a Service that connects to other validators within the same Validator Set.
// Don't forget to Start() and Stop() the service.
func NewValidatorConnExecutor(
	proTxHash types.ProTxHash,
	eventBus *types.EventBus,
	connMgr p2p.DashDialer,
	opts ...optionFunc,
) (*ValidatorConnExecutor, error) {
	vc := &ValidatorConnExecutor{
		proTxHash:           proTxHash,
		eventBus:            eventBus,
		dialer:              connMgr,
		EventBusCapacity:    defaultEventBusCapacity,
		validatorSetMembers: validatorMap{},
		connectedValidators: validatorMap{},
		quorumHash:          make(tmbytes.HexBytes, crypto.QuorumHashSize),
	}
	vc.nodeIDResolvers = []p2p.NodeIDResolver{
		vc.dialer,
		NewTCPNodeIDResolver(),
	}
	baseService := service.NewBaseService(log.NewNopLogger(), validatorConnExecutorName, vc)
	vc.BaseService = *baseService

	for _, opt := range opts {
		err := opt(vc)
		if err != nil {
			return nil, err
		}
	}
	return vc, nil
}

// WithValidatorsSet sets the validators and quorum-hash as default values
func WithValidatorsSet(valSet *types.ValidatorSet) func(vc *ValidatorConnExecutor) error {
	return func(vc *ValidatorConnExecutor) error {
		if len(valSet.Validators) == 0 {
			return nil
		}
		err := vc.setQuorumHash(valSet.QuorumHash)
		if err != nil {
			return err
		}
		vc.validatorSetMembers = newValidatorMap(valSet.Validators)
		return nil
	}
}

// WithLogger sets a logger
func WithLogger(logger log.Logger) func(vc *ValidatorConnExecutor) error {
	return func(vc *ValidatorConnExecutor) error {
		vc.Logger = logger
		return nil
	}
}

// OnStart implements Service to subscribe to Validator Update events
func (vc *ValidatorConnExecutor) OnStart() error {
	if err := vc.subscribe(); err != nil {
		return err
	}
	// establish connection with validators retrieves from genesis or init-chain
	err := vc.updateConnections()
	if err != nil {
		return err
	}
	go func() {
		var err error
		for err == nil {
			err = vc.receiveEvents()
		}
		vc.Logger.Error("ValidatorConnExecutor goroutine finished", "reason", err)
	}()
	return nil
}

// OnStop implements Service to clean up (unsubscribe from all events, stop timers, etc.)
func (vc *ValidatorConnExecutor) OnStop() {
	if vc.eventBus != nil {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()
		err := vc.eventBus.UnsubscribeAll(ctx, validatorConnExecutorName)
		if err != nil {
			vc.Logger.Error("cannot unsubscribe from channels", "error", err)
		}
		vc.eventBus = nil
	}
}

// subscribe subscribes to event bus to receive validator update messages
func (vc *ValidatorConnExecutor) subscribe() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	updatesSub, err := vc.eventBus.Subscribe(
		ctx,
		validatorConnExecutorName,
		types.EventQueryValidatorSetUpdates,
		vc.EventBusCapacity,
	)
	if err != nil {
		return err
	}

	vc.subscription = updatesSub
	return nil
}

// receiveEvents processes received events and executes all the logic.
// Returns non-nil error only if fatal error occurred and the main goroutine should be terminated.
func (vc *ValidatorConnExecutor) receiveEvents() error {
	vc.Logger.Debug("ValidatorConnExecutor: waiting for an event")
	select {
	case msg := <-vc.subscription.Out():
		event, ok := msg.Data().(types.EventDataValidatorSetUpdate)
		if !ok {
			return fmt.Errorf("invalid type of validator set update message: %T", event)
		}
		if err := vc.handleValidatorUpdateEvent(event); err != nil {
			vc.Logger.Error("cannot handle validator update", "error", err)
			return nil // non-fatal, so no error returned to continue the loop
		}
		vc.Logger.Debug("validator updates processed successfully", "event", event)
	case <-vc.subscription.Canceled():
		return fmt.Errorf("subscription canceled due to error: %w", vc.subscription.Err())
	case <-vc.BaseService.Quit():
		return fmt.Errorf("quit signal received")
	}

	return nil
}

// handleValidatorUpdateEvent checks and executes event of type EventDataValidatorSetUpdates, received from event bus.
func (vc *ValidatorConnExecutor) handleValidatorUpdateEvent(event types.EventDataValidatorSetUpdate) error {
	vc.mux.Lock()
	defer vc.mux.Unlock()

	if len(event.ValidatorSetUpdates) < 1 {
		vc.Logger.Debug("no validators in ValidatorUpdates")
		return nil // not really an error
	}
	vc.validatorSetMembers = newValidatorMap(event.ValidatorSetUpdates)
	if len(event.QuorumHash) > 0 {
		if err := vc.setQuorumHash(event.QuorumHash); err != nil {
			vc.Logger.Error("received invalid quorum hash", "error", err)
			return fmt.Errorf("received invalid quorum hash: %w", err)
		}
	} else {
		vc.Logger.Debug("received empty quorum hash")
	}
	if err := vc.updateConnections(); err != nil {
		return fmt.Errorf("inter-validator set connections error: %w", err)
	}
	return nil
}

// setQuorumHash sets quorum hash to provided bytes
func (vc *ValidatorConnExecutor) setQuorumHash(newQuorumHash tmbytes.HexBytes) error {
	// New quorum hash must be exactly `crypto.QuorumHashSize` bytes long
	if len(newQuorumHash) != crypto.QuorumHashSize {
		return fmt.Errorf("invalid quorum hash size: got %d, expected %d; quorum hash: %x",
			len(newQuorumHash), crypto.QuorumHashSize, newQuorumHash)
	}
	copy(vc.quorumHash, newQuorumHash)
	return nil
}

// me returns current node's validator object, if any.
// `ok` is false when current node is not a validator.
func (vc *ValidatorConnExecutor) me() (validator *types.Validator, ok bool) {
	v, ok := vc.validatorSetMembers[validatorMapIndexType(vc.proTxHash.String())]
	return &v, ok
}

// resolveNodeID adds node ID to the validator address if it's not set
func (vc *ValidatorConnExecutor) resolveNodeID(va *types.ValidatorAddress) error {
	if va.NodeID != "" {
		return nil
	}
	for _, resolver := range vc.nodeIDResolvers {
		address, err := resolver.Resolve(*va)
		if err == nil && address.NodeID != "" {
			va.NodeID = address.NodeID
			return nil
		}
		vc.Logger.Debug(
			"warning: validator node id lookup method failed",
			"url", va.String(),
			"method", reflect.TypeOf(resolver).String(),
			"error", err,
		)
	}
	return types.ErrNoNodeID
}

// selectValidators selects `count` validators from current ValidatorSet.
// It uses algorithm described in DIP-6.
// Returns map indexed by validator address.
func (vc *ValidatorConnExecutor) selectValidators() (validatorMap, error) {
	activeValidators := vc.validatorSetMembers
	me, ok := vc.me()
	if !ok {
		return validatorMap{}, fmt.Errorf("current node is not member of active validator set")
	}

	selector := selectpeers.NewDIP6ValidatorSelector(vc.quorumHash)
	selectedValidators, err := selector.SelectValidators(activeValidators.values(), me)
	if err != nil {
		return validatorMap{}, err
	}
	// fetch node IDs
	for _, validator := range selectedValidators {
		err := vc.resolveNodeID(&validator.NodeAddress)
		if err != nil {
			vc.Logger.Debug("cannot determine node id for validator", "url", validator.String(), "error", err)
			// no return, as it's not critical
		}
	}

	return newValidatorMap(selectedValidators), nil
}

func (vc *ValidatorConnExecutor) disconnectValidator(validator types.Validator) error {
	if err := vc.resolveNodeID(&validator.NodeAddress); err != nil {
		return err
	}
	id := validator.NodeAddress.NodeID
	vc.Logger.Debug("disconnecting Validator", "validator", validator, "id", id, "address", validator.NodeAddress.String())
	if err := vc.dialer.DisconnectAsync(id); err != nil {
		return err
	}
	return nil
}

// disconnectValidators disconnects connected validators which are not a part of the exceptions map
func (vc *ValidatorConnExecutor) disconnectValidators(exceptions validatorMap) error {
	for currentKey, validator := range vc.connectedValidators {
		if exceptions.contains(validator) {
			continue
		}
		if err := vc.disconnectValidator(validator); err != nil {
			if !errors.Is(err, errPeerNotFound) {
				// no return, as we see it as non-fatal
				vc.Logger.Error("cannot disconnect Validator", "error", err)
				continue
			}
			vc.Logger.Debug("Validator already disconnected", "error", err)
			// We still delete the validator from vc.connectedValidators
		}
		delete(vc.connectedValidators, currentKey)
	}

	return nil
}

// isValidator returns true when current node is a validator
func (vc *ValidatorConnExecutor) isValidator() bool {
	_, ok := vc.me()
	return ok
}

// updateConnections processes current validator set, selects a few validators and schedules connections
// to be established. It will also disconnect previous validators.
func (vc *ValidatorConnExecutor) updateConnections() error {
	// We only do something if we are part of new ValidatorSet
	if !vc.isValidator() {
		vc.Logger.Debug("not a member of active ValidatorSet")
		// We need to disconnect connected validators. It needs to be done explicitly
		// because they are marked as persistent and will never disconnect themselves.
		return vc.disconnectValidators(validatorMap{})
	}

	// Find new newValidators
	newValidators, err := vc.selectValidators()
	if err != nil {
		vc.Logger.Error("cannot determine list of validators to connect", "error", err)
		// no return, as we still need to disconnect unused validators
	}
	// Disconnect existing validators unless they are selected to be connected again
	if err := vc.disconnectValidators(newValidators); err != nil {
		return fmt.Errorf("cannot disconnect unused validators: %w", err)
	}
	vc.Logger.Debug("filtering validators", "validators", newValidators.String())
	// ensure that we can connect to all validators
	newValidators = vc.filterAddresses(newValidators)
	// Connect to new validators
	vc.Logger.Debug("dialing validators", "validators", newValidators.String())
	if err := vc.dial(newValidators); err != nil {
		return fmt.Errorf("cannot dial validators: %w", err)
	}
	vc.Logger.Debug("connected to Validators", "validators", newValidators.String())
	return nil
}

// filterValidators returns new validatorMap that contains only validators to which we can connect
func (vc *ValidatorConnExecutor) filterAddresses(validators validatorMap) validatorMap {
	filtered := make(validatorMap, len(validators))
	for id, validator := range validators {
		if err := validator.ValidateBasic(); err != nil {
			vc.Logger.Debug("validator address is invalid", "id", id, "address", validator.NodeAddress.String())
			continue
		}
		if vc.connectedValidators.contains(validator) {
			vc.Logger.Debug("validator already connected", "id", id)
			continue
		}
		if vc.dialer.IsDialingOrConnected(validator.NodeAddress.NodeID) {
			vc.Logger.Debug("already dialing this validator", "id", id, "address", validator.NodeAddress.String())
			continue
		}

		filtered[id] = validator
	}
	return filtered
}

// dial dials the validators and ensures they will be persistent
func (vc *ValidatorConnExecutor) dial(vals validatorMap) error {
	if len(vals) < 1 {
		return nil
	}

	// we mark all validators as connected, to disconnect it in future validator rotation even if sth went wrong
	for id, validator := range vals {
		vc.connectedValidators[id] = validator
		address := nodeAddress(validator.NodeAddress)
		if err := vc.dialer.ConnectAsync(address); err != nil {
			vc.Logger.Error("cannot dial validator", "address", address.String(), "err", err)
			return fmt.Errorf("cannot dial validator %s: %w", address.String(), err)
		}
	}

	return nil
}
