package state

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/events"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/vm"
)

// NOTE: If an error occurs during block execution, state will be left
// at an invalid state.  Copy the state before calling ExecBlock!
func ExecBlock(s *State, block *types.Block, blockPartsHeader types.PartSetHeader) error {
	err := execBlock(s, block, blockPartsHeader)
	if err != nil {
		return err
	}
	// State.Hash should match block.StateHash
	stateHash := s.Hash()
	if !bytes.Equal(stateHash, block.StateHash) {
		return fmt.Errorf("Invalid state hash. Expected %X, got %X",
			stateHash, block.StateHash)
	}
	return nil
}

// executes transactions of a block, does not check block.StateHash
// NOTE: If an error occurs during block execution, state will be left
// at an invalid state.  Copy the state before calling execBlock!
func execBlock(s *State, block *types.Block, blockPartsHeader types.PartSetHeader) error {
	// Basic block validation.
	err := block.ValidateBasic(s.ChainID, s.LastBlockHeight, s.LastBlockHash, s.LastBlockParts, s.LastBlockTime)
	if err != nil {
		return err
	}

	// Validate block Validation.
	if block.Height == 1 {
		if len(block.Validation.Precommits) != 0 {
			return errors.New("Block at height 1 (first block) should have no Validation precommits")
		}
	} else {
		if uint(len(block.Validation.Precommits)) != s.LastBondedValidators.Size() {
			return errors.New(Fmt("Invalid block validation size. Expected %v, got %v",
				s.LastBondedValidators.Size(), len(block.Validation.Precommits)))
		}
		var sumVotingPower uint64
		s.LastBondedValidators.Iterate(func(index uint, val *Validator) bool {
			precommit := block.Validation.Precommits[index]
			if precommit.IsZero() {
				return false
			} else {
				vote := &types.Vote{
					Height:     block.Height - 1,
					Round:      block.Validation.Round,
					Type:       types.VoteTypePrecommit,
					BlockHash:  block.LastBlockHash,
					BlockParts: block.LastBlockParts,
				}
				if val.PubKey.VerifyBytes(account.SignBytes(s.ChainID, vote), precommit.Signature) {
					sumVotingPower += val.VotingPower
					return false
				} else {
					log.Warn(Fmt("Invalid validation signature.\nval: %v\nvote: %v", val, vote))
					err = errors.New("Invalid validation signature")
					return true
				}
			}
		})
		if err != nil {
			return err
		}
		if sumVotingPower <= s.LastBondedValidators.TotalVotingPower()*2/3 {
			return errors.New("Insufficient validation voting power")
		}
	}

	// Update Validator.LastCommitHeight as necessary.
	for i, precommit := range block.Validation.Precommits {
		if precommit.IsZero() {
			continue
		}
		_, val := s.LastBondedValidators.GetByIndex(uint(i))
		if val == nil {
			panic(Fmt("Failed to fetch validator at index %v", i))
		}
		if _, val_ := s.BondedValidators.GetByAddress(val.Address); val_ != nil {
			val_.LastCommitHeight = block.Height - 1
			updated := s.BondedValidators.Update(val_)
			if !updated {
				panic("Failed to update bonded validator LastCommitHeight")
			}
		} else if _, val_ := s.UnbondingValidators.GetByAddress(val.Address); val_ != nil {
			val_.LastCommitHeight = block.Height - 1
			updated := s.UnbondingValidators.Update(val_)
			if !updated {
				panic("Failed to update unbonding validator LastCommitHeight")
			}
		} else {
			panic("Could not find validator")
		}
	}

	// Remember LastBondedValidators
	s.LastBondedValidators = s.BondedValidators.Copy()

	// Create BlockCache to cache changes to state.
	blockCache := NewBlockCache(s)

	// Execute each tx
	for _, tx := range block.Data.Txs {
		err := ExecTx(blockCache, tx, true, s.evc)
		if err != nil {
			return InvalidTxError{tx, err}
		}
	}

	// Now sync the BlockCache to the backend.
	blockCache.Sync()

	// If any unbonding periods are over,
	// reward account with bonded coins.
	toRelease := []*Validator{}
	s.UnbondingValidators.Iterate(func(index uint, val *Validator) bool {
		if val.UnbondHeight+unbondingPeriodBlocks < block.Height {
			toRelease = append(toRelease, val)
		}
		return false
	})
	for _, val := range toRelease {
		s.releaseValidator(val)
	}

	// If any validators haven't signed in a while,
	// unbond them, they have timed out.
	toTimeout := []*Validator{}
	s.BondedValidators.Iterate(func(index uint, val *Validator) bool {
		lastActivityHeight := MaxUint(val.BondHeight, val.LastCommitHeight)
		if lastActivityHeight+validatorTimeoutBlocks < block.Height {
			log.Info("Validator timeout", "validator", val, "height", block.Height)
			toTimeout = append(toTimeout, val)
		}
		return false
	})
	for _, val := range toTimeout {
		s.unbondValidator(val)
	}

	// Increment validator AccumPowers
	s.BondedValidators.IncrementAccum(1)
	s.LastBlockHeight = block.Height
	s.LastBlockHash = block.Hash()
	s.LastBlockParts = blockPartsHeader
	s.LastBlockTime = block.Time
	return nil
}

// The accounts from the TxInputs must either already have
// account.PubKey.(type) != nil, (it must be known),
// or it must be specified in the TxInput.  If redeclared,
// the TxInput is modified and input.PubKey set to nil.
func getOrMakeAccounts(state AccountGetter, ins []*types.TxInput, outs []*types.TxOutput) (map[string]*account.Account, error) {
	accounts := map[string]*account.Account{}
	for _, in := range ins {
		// Account shouldn't be duplicated
		if _, ok := accounts[string(in.Address)]; ok {
			return nil, types.ErrTxDuplicateAddress
		}
		acc := state.GetAccount(in.Address)
		if acc == nil {
			return nil, types.ErrTxInvalidAddress
		}
		// PubKey should be present in either "account" or "in"
		if err := checkInputPubKey(acc, in); err != nil {
			return nil, err
		}
		accounts[string(in.Address)] = acc
	}
	for _, out := range outs {
		// Account shouldn't be duplicated
		if _, ok := accounts[string(out.Address)]; ok {
			return nil, types.ErrTxDuplicateAddress
		}
		acc := state.GetAccount(out.Address)
		// output account may be nil (new)
		if acc == nil {
			acc = &account.Account{
				Address:  out.Address,
				PubKey:   nil,
				Sequence: 0,
				Balance:  0,
			}
		}
		accounts[string(out.Address)] = acc
	}
	return accounts, nil
}

func checkInputPubKey(acc *account.Account, in *types.TxInput) error {
	if acc.PubKey == nil {
		if in.PubKey == nil {
			return types.ErrTxUnknownPubKey
		}
		if !bytes.Equal(in.PubKey.Address(), acc.Address) {
			return types.ErrTxInvalidPubKey
		}
		acc.PubKey = in.PubKey
	} else {
		in.PubKey = nil
	}
	return nil
}

func validateInputs(accounts map[string]*account.Account, signBytes []byte, ins []*types.TxInput) (total uint64, err error) {
	for _, in := range ins {
		acc := accounts[string(in.Address)]
		if acc == nil {
			panic("validateInputs() expects account in accounts")
		}
		err = validateInput(acc, signBytes, in)
		if err != nil {
			return
		}
		// Good. Add amount to total
		total += in.Amount
	}
	return total, nil
}

func validateInput(acc *account.Account, signBytes []byte, in *types.TxInput) (err error) {
	// Check TxInput basic
	if err := in.ValidateBasic(); err != nil {
		return err
	}
	// Check signatures
	if !acc.PubKey.VerifyBytes(signBytes, in.Signature) {
		return types.ErrTxInvalidSignature
	}
	// Check sequences
	if acc.Sequence+1 != in.Sequence {
		return types.ErrTxInvalidSequence{
			Got:      uint64(in.Sequence),
			Expected: uint64(acc.Sequence + 1),
		}
	}
	// Check amount
	if acc.Balance < in.Amount {
		return types.ErrTxInsufficientFunds
	}
	return nil
}

func validateOutputs(outs []*types.TxOutput) (total uint64, err error) {
	for _, out := range outs {
		// Check TxOutput basic
		if err := out.ValidateBasic(); err != nil {
			return 0, err
		}
		// Good. Add amount to total
		total += out.Amount
	}
	return total, nil
}

func adjustByInputs(accounts map[string]*account.Account, ins []*types.TxInput) {
	for _, in := range ins {
		acc := accounts[string(in.Address)]
		if acc == nil {
			panic("adjustByInputs() expects account in accounts")
		}
		if acc.Balance < in.Amount {
			panic("adjustByInputs() expects sufficient funds")
		}
		acc.Balance -= in.Amount
		acc.Sequence += 1
	}
}

func adjustByOutputs(accounts map[string]*account.Account, outs []*types.TxOutput) {
	for _, out := range outs {
		acc := accounts[string(out.Address)]
		if acc == nil {
			panic("adjustByOutputs() expects account in accounts")
		}
		acc.Balance += out.Amount
	}
}

// If the tx is invalid, an error will be returned.
// Unlike ExecBlock(), state will not be altered.
func ExecTx(blockCache *BlockCache, tx_ types.Tx, runCall bool, evc events.Fireable) error {

	// TODO: do something with fees
	fees := uint64(0)
	_s := blockCache.State() // hack to access validators and block height

	// Exec tx
	switch tx := tx_.(type) {
	case *types.SendTx:
		accounts, err := getOrMakeAccounts(blockCache, tx.Inputs, tx.Outputs)
		if err != nil {
			return err
		}
		signBytes := account.SignBytes(_s.ChainID, tx)
		inTotal, err := validateInputs(accounts, signBytes, tx.Inputs)
		if err != nil {
			return err
		}
		outTotal, err := validateOutputs(tx.Outputs)
		if err != nil {
			return err
		}
		if outTotal > inTotal {
			return types.ErrTxInsufficientFunds
		}
		fee := inTotal - outTotal
		fees += fee

		// Good! Adjust accounts
		adjustByInputs(accounts, tx.Inputs)
		adjustByOutputs(accounts, tx.Outputs)
		for _, acc := range accounts {
			blockCache.UpdateAccount(acc)
		}

		// if the evc is nil, nothing will happen
		if evc != nil {
			for _, i := range tx.Inputs {
				evc.FireEvent(types.EventStringAccInput(i.Address), tx)
			}

			for _, o := range tx.Outputs {
				evc.FireEvent(types.EventStringAccOutput(o.Address), tx)
			}
		}
		return nil

	case *types.CallTx:
		var inAcc, outAcc *account.Account

		// Validate input
		inAcc = blockCache.GetAccount(tx.Input.Address)
		if inAcc == nil {
			log.Debug(Fmt("Can't find in account %X", tx.Input.Address))
			return types.ErrTxInvalidAddress
		}
		// pubKey should be present in either "inAcc" or "tx.Input"
		if err := checkInputPubKey(inAcc, tx.Input); err != nil {
			log.Debug(Fmt("Can't find pubkey for %X", tx.Input.Address))
			return err
		}
		signBytes := account.SignBytes(_s.ChainID, tx)
		err := validateInput(inAcc, signBytes, tx.Input)
		if err != nil {
			log.Debug(Fmt("validateInput failed on %X: %v", tx.Input.Address, err))
			return err
		}
		if tx.Input.Amount < tx.Fee {
			log.Debug(Fmt("Sender did not send enough to cover the fee %X", tx.Input.Address))
			return types.ErrTxInsufficientFunds
		}

		createAccount := len(tx.Address) == 0
		if !createAccount {
			// Validate output
			if len(tx.Address) != 20 {
				log.Debug(Fmt("Destination address is not 20 bytes %X", tx.Address))
				return types.ErrTxInvalidAddress
			}
			// this may be nil if we are still in mempool and contract was created in same block as this tx
			// but that's fine, because the account will be created properly when the create tx runs in the block
			// and then this won't return nil. otherwise, we take their fee
			outAcc = blockCache.GetAccount(tx.Address)
		}

		log.Debug(Fmt("Out account: %v", outAcc))

		// Good!
		value := tx.Input.Amount - tx.Fee
		inAcc.Sequence += 1

		if runCall {

			var (
				gas     uint64      = tx.GasLimit
				err     error       = nil
				caller  *vm.Account = toVMAccount(inAcc)
				callee  *vm.Account = nil
				code    []byte      = nil
				txCache             = NewTxCache(blockCache)
				params              = vm.Params{
					BlockHeight: uint64(_s.LastBlockHeight),
					BlockHash:   LeftPadWord256(_s.LastBlockHash),
					BlockTime:   _s.LastBlockTime.Unix(),
					GasLimit:    10000000,
				}
			)

			// Maybe create a new callee account if
			// this transaction is creating a new contract.
			if !createAccount {
				if outAcc == nil || len(outAcc.Code) == 0 {
					// if you call an account that doesn't exist
					// or an account with no code then we take fees (sorry pal)
					// NOTE: it's fine to create a contract and call it within one
					// block (nonce will prevent re-ordering of those txs)
					// but to create with one account and call with another
					// you have to wait a block to avoid a re-ordering attack
					// that will take your fees
					inAcc.Balance -= tx.Fee
					blockCache.UpdateAccount(inAcc)
					if outAcc == nil {
						log.Debug(Fmt("Cannot find destination address %X. Deducting fee from caller", tx.Address))
					} else {
						log.Debug(Fmt("Attempting to call an account (%X) with no code. Deducting fee from caller", tx.Address))
					}
					return types.ErrTxInvalidAddress

				}
				callee = toVMAccount(outAcc)
				code = callee.Code
				log.Debug(Fmt("Calling contract %X with code %X", callee.Address, callee.Code))
			} else {
				callee = txCache.CreateAccount(caller)
				log.Debug(Fmt("Created new account %X", callee.Address))
				code = tx.Data
			}
			log.Debug(Fmt("Code for this contract: %X", code))

			txCache.UpdateAccount(caller) // because we adjusted by input above, and bumped nonce maybe.
			txCache.UpdateAccount(callee) // because we adjusted by input above.
			vmach := vm.NewVM(txCache, params, caller.Address, account.HashSignBytes(_s.ChainID, tx))
			vmach.SetFireable(evc)
			// NOTE: Call() transfers the value from caller to callee iff call succeeds.

			ret, err := vmach.Call(caller, callee, code, tx.Data, value, &gas)
			exception := ""
			if err != nil {
				exception = err.Error()
				// Failure. Charge the gas fee. The 'value' was otherwise not transferred.
				log.Debug(Fmt("Error on execution: %v", err))
				inAcc.Balance -= tx.Fee
				blockCache.UpdateAccount(inAcc)
				// Throw away 'txCache' which holds incomplete updates (don't sync it).
			} else {
				log.Debug("Successful execution")
				// Success
				if createAccount {
					callee.Code = ret
				}

				txCache.Sync()
			}
			// Create a receipt from the ret and whether errored.
			log.Info("VM call complete", "caller", caller, "callee", callee, "return", ret, "err", err)

			// Fire Events for sender and receiver
			// a separate event will be fired from vm for each additional call
			if evc != nil {
				evc.FireEvent(types.EventStringAccInput(tx.Input.Address), types.EventMsgCallTx{tx, ret, exception})
				evc.FireEvent(types.EventStringAccOutput(tx.Address), types.EventMsgCallTx{tx, ret, exception})
			}
		} else {
			// The mempool does not call txs until
			// the proposer determines the order of txs.
			// So mempool will skip the actual .Call(),
			// and only deduct from the caller's balance.
			inAcc.Balance -= value
			if createAccount {
				inAcc.Sequence += 1
			}
			blockCache.UpdateAccount(inAcc)
		}

		return nil

	case *types.NameTx:
		var inAcc *account.Account

		// Validate input
		inAcc = blockCache.GetAccount(tx.Input.Address)
		if inAcc == nil {
			log.Debug(Fmt("Can't find in account %X", tx.Input.Address))
			return types.ErrTxInvalidAddress
		}
		// pubKey should be present in either "inAcc" or "tx.Input"
		if err := checkInputPubKey(inAcc, tx.Input); err != nil {
			log.Debug(Fmt("Can't find pubkey for %X", tx.Input.Address))
			return err
		}
		signBytes := account.SignBytes(_s.ChainID, tx)
		err := validateInput(inAcc, signBytes, tx.Input)
		if err != nil {
			log.Debug(Fmt("validateInput failed on %X: %v", tx.Input.Address, err))
			return err
		}
		// fee is in addition to the amount which is used to determine the TTL
		if tx.Input.Amount < tx.Fee {
			log.Debug(Fmt("Sender did not send enough to cover the fee %X", tx.Input.Address))
			return types.ErrTxInsufficientFunds
		}

		// validate the input strings
		if err := tx.ValidateStrings(); err != nil {
			log.Debug(err.Error())
			return types.ErrTxInvalidString
		}

		value := tx.Input.Amount - tx.Fee

		// let's say cost of a name for one block is len(data) + 32
		costPerBlock := types.NameCostPerBlock * types.NameCostPerByte * tx.BaseEntryCost()
		expiresIn := value / uint64(costPerBlock)
		lastBlockHeight := uint64(_s.LastBlockHeight)

		log.Debug("New NameTx", "value", value, "costPerBlock", costPerBlock, "expiresIn", expiresIn, "lastBlock", lastBlockHeight)

		// check if the name exists
		entry := blockCache.GetNameRegEntry(tx.Name)

		if entry != nil {
			var expired bool
			// if the entry already exists, and hasn't expired, we must be owner
			if entry.Expires > lastBlockHeight {
				// ensure we are owner
				if bytes.Compare(entry.Owner, tx.Input.Address) != 0 {
					log.Debug(Fmt("Sender %X is trying to update a name (%s) for which he is not owner", tx.Input.Address, tx.Name))
					return types.ErrIncorrectOwner
				}
			} else {
				expired = true
			}

			// no value and empty data means delete the entry
			if value == 0 && len(tx.Data) == 0 {
				// maybe we reward you for telling us we can delete this crap
				// (owners if not expired, anyone if expired)
				log.Debug("Removing namereg entry", "name", entry.Name)
				blockCache.RemoveNameRegEntry(entry.Name)
			} else {
				// update the entry by bumping the expiry
				// and changing the data
				if expired {
					if expiresIn < types.MinNameRegistrationPeriod {
						return fmt.Errorf("Names must be registered for at least %d blocks", types.MinNameRegistrationPeriod)
					}
					entry.Expires = lastBlockHeight + expiresIn
					entry.Owner = tx.Input.Address
					log.Debug("An old namereg entry has expired and been reclaimed", "name", entry.Name, "expiresIn", expiresIn, "owner", entry.Owner)
				} else {
					// since the size of the data may have changed
					// we use the total amount of "credit"
					oldCredit := (entry.Expires - lastBlockHeight) * types.BaseEntryCost(entry.Name, entry.Data)
					credit := oldCredit + value
					expiresIn = credit / costPerBlock
					if expiresIn < types.MinNameRegistrationPeriod {
						return fmt.Errorf("Names must be registered for at least %d blocks", types.MinNameRegistrationPeriod)
					}
					entry.Expires = lastBlockHeight + expiresIn
					log.Debug("Updated namereg entry", "name", entry.Name, "expiresIn", expiresIn, "oldCredit", oldCredit, "value", value, "credit", credit)
				}
				entry.Data = tx.Data
				blockCache.UpdateNameRegEntry(entry)
			}
		} else {
			if expiresIn < types.MinNameRegistrationPeriod {
				return fmt.Errorf("Names must be registered for at least %d blocks", types.MinNameRegistrationPeriod)
			}
			// entry does not exist, so create it
			entry = &types.NameRegEntry{
				Name:    tx.Name,
				Owner:   tx.Input.Address,
				Data:    tx.Data,
				Expires: lastBlockHeight + expiresIn,
			}
			log.Debug("Creating namereg entry", "name", entry.Name, "expiresIn", expiresIn)
			blockCache.UpdateNameRegEntry(entry)
		}

		// TODO: something with the value sent?

		// Good!
		inAcc.Sequence += 1
		inAcc.Balance -= value
		blockCache.UpdateAccount(inAcc)

		// TODO: maybe we want to take funds on error and allow txs in that don't do anythingi?

		return nil

	case *types.BondTx:
		valInfo := blockCache.State().GetValidatorInfo(tx.PubKey.Address())
		if valInfo != nil {
			// TODO: In the future, check that the validator wasn't destroyed,
			// add funds, merge UnbondTo outputs, and unbond validator.
			return errors.New("Adding coins to existing validators not yet supported")
		}
		accounts, err := getOrMakeAccounts(blockCache, tx.Inputs, nil)
		if err != nil {
			return err
		}

		signBytes := account.SignBytes(_s.ChainID, tx)
		inTotal, err := validateInputs(accounts, signBytes, tx.Inputs)
		if err != nil {
			return err
		}
		if err := tx.PubKey.ValidateBasic(); err != nil {
			return err
		}
		if !tx.PubKey.VerifyBytes(signBytes, tx.Signature) {
			return types.ErrTxInvalidSignature
		}
		outTotal, err := validateOutputs(tx.UnbondTo)
		if err != nil {
			return err
		}
		if outTotal > inTotal {
			return types.ErrTxInsufficientFunds
		}
		fee := inTotal - outTotal
		fees += fee

		// Good! Adjust accounts
		adjustByInputs(accounts, tx.Inputs)
		for _, acc := range accounts {
			blockCache.UpdateAccount(acc)
		}
		// Add ValidatorInfo
		_s.SetValidatorInfo(&ValidatorInfo{
			Address:         tx.PubKey.Address(),
			PubKey:          tx.PubKey,
			UnbondTo:        tx.UnbondTo,
			FirstBondHeight: _s.LastBlockHeight + 1,
			FirstBondAmount: outTotal,
		})
		// Add Validator
		added := _s.BondedValidators.Add(&Validator{
			Address:     tx.PubKey.Address(),
			PubKey:      tx.PubKey,
			BondHeight:  _s.LastBlockHeight + 1,
			VotingPower: outTotal,
			Accum:       0,
		})
		if !added {
			panic("Failed to add validator")
		}
		if evc != nil {
			evc.FireEvent(types.EventStringBond(), tx)
		}
		return nil

	case *types.UnbondTx:
		// The validator must be active
		_, val := _s.BondedValidators.GetByAddress(tx.Address)
		if val == nil {
			return types.ErrTxInvalidAddress
		}

		// Verify the signature
		signBytes := account.SignBytes(_s.ChainID, tx)
		if !val.PubKey.VerifyBytes(signBytes, tx.Signature) {
			return types.ErrTxInvalidSignature
		}

		// tx.Height must be greater than val.LastCommitHeight
		if tx.Height <= val.LastCommitHeight {
			return errors.New("Invalid unbond height")
		}

		// Good!
		_s.unbondValidator(val)
		if evc != nil {
			evc.FireEvent(types.EventStringUnbond(), tx)
		}
		return nil

	case *types.RebondTx:
		// The validator must be inactive
		_, val := _s.UnbondingValidators.GetByAddress(tx.Address)
		if val == nil {
			return types.ErrTxInvalidAddress
		}

		// Verify the signature
		signBytes := account.SignBytes(_s.ChainID, tx)
		if !val.PubKey.VerifyBytes(signBytes, tx.Signature) {
			return types.ErrTxInvalidSignature
		}

		// tx.Height must be equal to the next height
		if tx.Height != _s.LastBlockHeight+1 {
			return errors.New(Fmt("Invalid rebond height.  Expected %v, got %v", _s.LastBlockHeight+1, tx.Height))
		}

		// Good!
		_s.rebondValidator(val)
		if evc != nil {
			evc.FireEvent(types.EventStringRebond(), tx)
		}
		return nil

	case *types.DupeoutTx:
		// Verify the signatures
		_, accused := _s.BondedValidators.GetByAddress(tx.Address)
		if accused == nil {
			_, accused = _s.UnbondingValidators.GetByAddress(tx.Address)
			if accused == nil {
				return types.ErrTxInvalidAddress
			}
		}
		voteASignBytes := account.SignBytes(_s.ChainID, &tx.VoteA)
		voteBSignBytes := account.SignBytes(_s.ChainID, &tx.VoteB)
		if !accused.PubKey.VerifyBytes(voteASignBytes, tx.VoteA.Signature) ||
			!accused.PubKey.VerifyBytes(voteBSignBytes, tx.VoteB.Signature) {
			return types.ErrTxInvalidSignature
		}

		// Verify equivocation
		// TODO: in the future, just require one vote from a previous height that
		// doesn't exist on this chain.
		if tx.VoteA.Height != tx.VoteB.Height {
			return errors.New("DupeoutTx heights don't match")
		}
		if tx.VoteA.Round != tx.VoteB.Round {
			return errors.New("DupeoutTx rounds don't match")
		}
		if tx.VoteA.Type != tx.VoteB.Type {
			return errors.New("DupeoutTx types don't match")
		}
		if bytes.Equal(tx.VoteA.BlockHash, tx.VoteB.BlockHash) {
			return errors.New("DupeoutTx blockhashes shouldn't match")
		}

		// Good! (Bad validator!)
		_s.destroyValidator(accused)
		if evc != nil {
			evc.FireEvent(types.EventStringDupeout(), tx)
		}
		return nil

	default:
		panic("Unknown Tx type")
	}
}
