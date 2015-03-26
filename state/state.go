package state

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	dbm "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/merkle"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/vm"
)

var (
	stateKey                     = []byte("stateKey")
	minBondAmount                = uint64(1)           // TODO adjust
	defaultAccountsCacheCapacity = 1000                // TODO adjust
	unbondingPeriodBlocks        = uint(60 * 24 * 365) // TODO probably better to make it time based.
	validatorTimeoutBlocks       = uint(10)            // TODO adjust
)

//-----------------------------------------------------------------------------

type InvalidTxError struct {
	Tx     types.Tx
	Reason error
}

func (txErr InvalidTxError) Error() string {
	return fmt.Sprintf("Invalid tx: [%v] reason: [%v]", txErr.Tx, txErr.Reason)
}

//-----------------------------------------------------------------------------

// NOTE: not goroutine-safe.
type State struct {
	DB                   dbm.DB
	LastBlockHeight      uint
	LastBlockHash        []byte
	LastBlockParts       types.PartSetHeader
	LastBlockTime        time.Time
	BondedValidators     *ValidatorSet
	LastBondedValidators *ValidatorSet
	UnbondingValidators  *ValidatorSet
	accounts             merkle.Tree // Shouldn't be accessed directly.
	validatorInfos       merkle.Tree // Shouldn't be accessed directly.
}

func LoadState(db dbm.DB) *State {
	s := &State{DB: db}
	buf := db.Get(stateKey)
	if len(buf) == 0 {
		return nil
	} else {
		r, n, err := bytes.NewReader(buf), new(int64), new(error)
		s.LastBlockHeight = binary.ReadUvarint(r, n, err)
		s.LastBlockHash = binary.ReadByteSlice(r, n, err)
		s.LastBlockParts = binary.ReadBinary(types.PartSetHeader{}, r, n, err).(types.PartSetHeader)
		s.LastBlockTime = binary.ReadTime(r, n, err)
		s.BondedValidators = binary.ReadBinary(&ValidatorSet{}, r, n, err).(*ValidatorSet)
		s.LastBondedValidators = binary.ReadBinary(&ValidatorSet{}, r, n, err).(*ValidatorSet)
		s.UnbondingValidators = binary.ReadBinary(&ValidatorSet{}, r, n, err).(*ValidatorSet)
		accountsHash := binary.ReadByteSlice(r, n, err)
		s.accounts = merkle.NewIAVLTree(binary.BasicCodec, account.AccountCodec, defaultAccountsCacheCapacity, db)
		s.accounts.Load(accountsHash)
		validatorInfosHash := binary.ReadByteSlice(r, n, err)
		s.validatorInfos = merkle.NewIAVLTree(binary.BasicCodec, ValidatorInfoCodec, 0, db)
		s.validatorInfos.Load(validatorInfosHash)
		if *err != nil {
			panic(*err)
		}
		// TODO: ensure that buf is completely read.
	}
	return s
}

// Save this state into the db.
func (s *State) Save() {
	s.accounts.Save()
	s.validatorInfos.Save()
	buf, n, err := new(bytes.Buffer), new(int64), new(error)
	binary.WriteUvarint(s.LastBlockHeight, buf, n, err)
	binary.WriteByteSlice(s.LastBlockHash, buf, n, err)
	binary.WriteBinary(s.LastBlockParts, buf, n, err)
	binary.WriteTime(s.LastBlockTime, buf, n, err)
	binary.WriteBinary(s.BondedValidators, buf, n, err)
	binary.WriteBinary(s.LastBondedValidators, buf, n, err)
	binary.WriteBinary(s.UnbondingValidators, buf, n, err)
	binary.WriteByteSlice(s.accounts.Hash(), buf, n, err)
	binary.WriteByteSlice(s.validatorInfos.Hash(), buf, n, err)
	if *err != nil {
		panic(*err)
	}
	s.DB.Set(stateKey, buf.Bytes())
}

func (s *State) Copy() *State {
	return &State{
		DB:                   s.DB,
		LastBlockHeight:      s.LastBlockHeight,
		LastBlockHash:        s.LastBlockHash,
		LastBlockParts:       s.LastBlockParts,
		LastBlockTime:        s.LastBlockTime,
		BondedValidators:     s.BondedValidators.Copy(),     // TODO remove need for Copy() here.
		LastBondedValidators: s.LastBondedValidators.Copy(), // That is, make updates to the validator set
		UnbondingValidators:  s.UnbondingValidators.Copy(),  // copy the valSet lazily.
		accounts:             s.accounts.Copy(),
		validatorInfos:       s.validatorInfos.Copy(),
	}
}

// The accounts from the TxInputs must either already have
// account.PubKey.(type) != PubKeyNil, (it must be known),
// or it must be specified in the TxInput.  If redeclared,
// the TxInput is modified and input.PubKey set to PubKeyNil.
func (s *State) GetOrMakeAccounts(ins []*types.TxInput, outs []*types.TxOutput) (map[string]*account.Account, error) {
	accounts := map[string]*account.Account{}
	for _, in := range ins {
		// Account shouldn't be duplicated
		if _, ok := accounts[string(in.Address)]; ok {
			return nil, types.ErrTxDuplicateAddress
		}
		acc := s.GetAccount(in.Address)
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
		acc := s.GetAccount(out.Address)
		// output account may be nil (new)
		if acc == nil {
			acc = &account.Account{
				Address:  out.Address,
				PubKey:   account.PubKeyNil{},
				Sequence: 0,
				Balance:  0,
			}
		}
		accounts[string(out.Address)] = acc
	}
	return accounts, nil
}

func checkInputPubKey(acc *account.Account, in *types.TxInput) error {
	if _, isNil := acc.PubKey.(account.PubKeyNil); isNil {
		if _, isNil := in.PubKey.(account.PubKeyNil); isNil {
			return types.ErrTxUnknownPubKey
		}
		if !bytes.Equal(in.PubKey.Address(), acc.Address) {
			return types.ErrTxInvalidPubKey
		}
		acc.PubKey = in.PubKey
	} else {
		in.PubKey = account.PubKeyNil{}
	}
	return nil
}

func (s *State) ValidateInputs(accounts map[string]*account.Account, signBytes []byte, ins []*types.TxInput) (total uint64, err error) {
	for _, in := range ins {
		acc := accounts[string(in.Address)]
		if acc == nil {
			panic("ValidateInputs() expects account in accounts")
		}
		err = s.ValidateInput(acc, signBytes, in)
		if err != nil {
			return
		}
		// Good. Add amount to total
		total += in.Amount
	}
	return total, nil
}

func (s *State) ValidateInput(acc *account.Account, signBytes []byte, in *types.TxInput) (err error) {
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

func (s *State) ValidateOutputs(outs []*types.TxOutput) (total uint64, err error) {
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

func (s *State) AdjustByInputs(accounts map[string]*account.Account, ins []*types.TxInput) {
	for _, in := range ins {
		acc := accounts[string(in.Address)]
		if acc == nil {
			panic("AdjustByInputs() expects account in accounts")
		}
		if acc.Balance < in.Amount {
			panic("AdjustByInputs() expects sufficient funds")
		}
		acc.Balance -= in.Amount
		acc.Sequence += 1
	}
}

func (s *State) AdjustByOutputs(accounts map[string]*account.Account, outs []*types.TxOutput) {
	for _, out := range outs {
		acc := accounts[string(out.Address)]
		if acc == nil {
			panic("AdjustByOutputs() expects account in accounts")
		}
		acc.Balance += out.Amount
	}
}

// If the tx is invalid, an error will be returned.
// Unlike AppendBlock(), state will not be altered.
func (s *State) ExecTx(tx_ types.Tx, runCall bool) error {

	// TODO: do something with fees
	fees := uint64(0)

	// Exec tx
	switch tx := tx_.(type) {
	case *types.SendTx:
		accounts, err := s.GetOrMakeAccounts(tx.Inputs, tx.Outputs)
		if err != nil {
			return err
		}
		signBytes := account.SignBytes(tx)
		inTotal, err := s.ValidateInputs(accounts, signBytes, tx.Inputs)
		if err != nil {
			return err
		}
		outTotal, err := s.ValidateOutputs(tx.Outputs)
		if err != nil {
			return err
		}
		if outTotal > inTotal {
			return types.ErrTxInsufficientFunds
		}
		fee := inTotal - outTotal
		fees += fee

		// Good! Adjust accounts
		s.AdjustByInputs(accounts, tx.Inputs)
		s.AdjustByOutputs(accounts, tx.Outputs)
		s.UpdateAccounts(accounts)
		return nil

	case *types.CallTx:
		var inAcc, outAcc *account.Account

		// Validate input
		inAcc = s.GetAccount(tx.Input.Address)
		if inAcc == nil {
			log.Debug(Fmt("Can't find in account %X", tx.Input.Address))
			return types.ErrTxInvalidAddress
		}
		// pubKey should be present in either "inAcc" or "tx.Input"
		if err := checkInputPubKey(inAcc, tx.Input); err != nil {
			log.Debug(Fmt("Can't find pubkey for %X", tx.Input.Address))
			return err
		}
		signBytes := account.SignBytes(tx)
		err := s.ValidateInput(inAcc, signBytes, tx.Input)
		if err != nil {
			log.Debug(Fmt("ValidateInput failed on %X:", tx.Input.Address))
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
			outAcc = s.GetAccount(tx.Address)
		}

		log.Debug(Fmt("Out account: %v", outAcc))

		// Good!
		value := tx.Input.Amount - tx.Fee
		inAcc.Sequence += 1

		if runCall {

			var (
				gas      uint64      = tx.GasLimit
				err      error       = nil
				caller   *vm.Account = toVMAccount(inAcc)
				callee   *vm.Account = nil
				code     []byte      = nil
				appState             = NewVMAppState(s) // TODO: confusing.
				params               = vm.Params{
					BlockHeight: uint64(s.LastBlockHeight),
					BlockHash:   vm.BytesToWord(s.LastBlockHash),
					BlockTime:   s.LastBlockTime.Unix(),
					GasLimit:    10000000,
				}
			)

			// Maybe create a new callee account if
			// this transaction is creating a new contract.
			if !createAccount {
				if outAcc == nil {
					// take fees (sorry pal)
					inAcc.Balance -= tx.Fee
					s.UpdateAccount(inAcc)
					log.Debug(Fmt("Cannot find destination address %X. Deducting fee from caller", tx.Address))
					return types.ErrTxInvalidAddress

				}
				callee = toVMAccount(outAcc)
				code = callee.Code
				log.Debug(Fmt("Calling contract %X with code %X", callee.Address.Address(), callee.Code))
			} else {
				callee, err = appState.CreateAccount(caller)
				if err != nil {
					log.Debug(Fmt("Error creating account"))
					return err
				}
				log.Debug(Fmt("Created new account %X", callee.Address.Address()))
				code = tx.Data
			}
			log.Debug(Fmt("Code for this contract: %X", code))

			appState.UpdateAccount(caller) // because we adjusted by input above, and bumped nonce maybe.
			appState.UpdateAccount(callee) // because we adjusted by input above.
			vmach := vm.NewVM(appState, params, caller.Address)
			// NOTE: Call() transfers the value from caller to callee iff call succeeds.
			ret, err := vmach.Call(caller, callee, code, tx.Data, value, &gas)
			if err != nil {
				// Failure. Charge the gas fee. The 'value' was otherwise not transferred.
				log.Debug(Fmt("Error on execution: %v", err))
				inAcc.Balance -= tx.Fee
				s.UpdateAccount(inAcc)
				// Throw away 'appState' which holds incomplete updates (don't sync it).
			} else {
				log.Debug("Successful execution")
				// Success
				if createAccount {
					callee.Code = ret
				}

				appState.Sync()
			}
			// Create a receipt from the ret and whether errored.
			log.Info("VM call complete", "caller", caller, "callee", callee, "return", ret, "err", err)
		} else {
			// The mempool does not call txs until
			// the proposer determines the order of txs.
			// So mempool will skip the actual .Call(),
			// and only deduct from the caller's balance.
			inAcc.Balance -= value
			if createAccount {
				inAcc.Sequence += 1
			}
			s.UpdateAccount(inAcc)
		}

		return nil

	case *types.BondTx:
		valInfo := s.GetValidatorInfo(tx.PubKey.Address())
		if valInfo != nil {
			// TODO: In the future, check that the validator wasn't destroyed,
			// add funds, merge UnbondTo outputs, and unbond validator.
			return errors.New("Adding coins to existing validators not yet supported")
		}
		accounts, err := s.GetOrMakeAccounts(tx.Inputs, nil)
		if err != nil {
			return err
		}
		signBytes := account.SignBytes(tx)
		inTotal, err := s.ValidateInputs(accounts, signBytes, tx.Inputs)
		if err != nil {
			return err
		}
		if err := tx.PubKey.ValidateBasic(); err != nil {
			return err
		}
		outTotal, err := s.ValidateOutputs(tx.UnbondTo)
		if err != nil {
			return err
		}
		if outTotal > inTotal {
			return types.ErrTxInsufficientFunds
		}
		fee := inTotal - outTotal
		fees += fee

		// Good! Adjust accounts
		s.AdjustByInputs(accounts, tx.Inputs)
		s.UpdateAccounts(accounts)
		// Add ValidatorInfo
		s.SetValidatorInfo(&ValidatorInfo{
			Address:         tx.PubKey.Address(),
			PubKey:          tx.PubKey,
			UnbondTo:        tx.UnbondTo,
			FirstBondHeight: s.LastBlockHeight + 1,
			FirstBondAmount: outTotal,
		})
		// Add Validator
		added := s.BondedValidators.Add(&Validator{
			Address:     tx.PubKey.Address(),
			PubKey:      tx.PubKey,
			BondHeight:  s.LastBlockHeight + 1,
			VotingPower: outTotal,
			Accum:       0,
		})
		if !added {
			panic("Failed to add validator")
		}
		return nil

	case *types.UnbondTx:
		// The validator must be active
		_, val := s.BondedValidators.GetByAddress(tx.Address)
		if val == nil {
			return types.ErrTxInvalidAddress
		}

		// Verify the signature
		signBytes := account.SignBytes(tx)
		if !val.PubKey.VerifyBytes(signBytes, tx.Signature) {
			return types.ErrTxInvalidSignature
		}

		// tx.Height must be greater than val.LastCommitHeight
		if tx.Height <= val.LastCommitHeight {
			return errors.New("Invalid unbond height")
		}

		// Good!
		s.unbondValidator(val)
		return nil

	case *types.RebondTx:
		// The validator must be inactive
		_, val := s.UnbondingValidators.GetByAddress(tx.Address)
		if val == nil {
			return types.ErrTxInvalidAddress
		}

		// Verify the signature
		signBytes := account.SignBytes(tx)
		if !val.PubKey.VerifyBytes(signBytes, tx.Signature) {
			return types.ErrTxInvalidSignature
		}

		// tx.Height must be equal to the next height
		if tx.Height != s.LastBlockHeight+1 {
			return errors.New(Fmt("Invalid rebond height.  Expected %v, got %v", s.LastBlockHeight+1, tx.Height))
		}

		// Good!
		s.rebondValidator(val)
		return nil

	case *types.DupeoutTx:
		// Verify the signatures
		_, accused := s.BondedValidators.GetByAddress(tx.Address)
		if accused == nil {
			_, accused = s.UnbondingValidators.GetByAddress(tx.Address)
			if accused == nil {
				return types.ErrTxInvalidAddress
			}
		}
		voteASignBytes := account.SignBytes(&tx.VoteA)
		voteBSignBytes := account.SignBytes(&tx.VoteB)
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
		if tx.VoteA.Type == types.VoteTypeCommit && tx.VoteA.Round < tx.VoteB.Round {
			// Check special case (not an error, validator must be slashed!)
			// Validators should not sign another vote after committing.
		} else if tx.VoteB.Type == types.VoteTypeCommit && tx.VoteB.Round < tx.VoteA.Round {
			// We need to check both orderings of the votes
		} else {
			if tx.VoteA.Round != tx.VoteB.Round {
				return errors.New("DupeoutTx rounds don't match")
			}
			if tx.VoteA.Type != tx.VoteB.Type {
				return errors.New("DupeoutTx types don't match")
			}
			if bytes.Equal(tx.VoteA.BlockHash, tx.VoteB.BlockHash) {
				return errors.New("DupeoutTx blockhashes shouldn't match")
			}
		}

		// Good! (Bad validator!)
		s.destroyValidator(accused)
		return nil

	default:
		panic("Unknown Tx type")
	}
}

func (s *State) unbondValidator(val *Validator) {
	// Move validator to UnbondingValidators
	val, removed := s.BondedValidators.Remove(val.Address)
	if !removed {
		panic("Couldn't remove validator for unbonding")
	}
	val.UnbondHeight = s.LastBlockHeight + 1
	added := s.UnbondingValidators.Add(val)
	if !added {
		panic("Couldn't add validator for unbonding")
	}
}

func (s *State) rebondValidator(val *Validator) {
	// Move validator to BondingValidators
	val, removed := s.UnbondingValidators.Remove(val.Address)
	if !removed {
		panic("Couldn't remove validator for rebonding")
	}
	val.BondHeight = s.LastBlockHeight + 1
	added := s.BondedValidators.Add(val)
	if !added {
		panic("Couldn't add validator for rebonding")
	}
}

func (s *State) releaseValidator(val *Validator) {
	// Update validatorInfo
	valInfo := s.GetValidatorInfo(val.Address)
	if valInfo == nil {
		panic("Couldn't find validatorInfo for release")
	}
	valInfo.ReleasedHeight = s.LastBlockHeight + 1
	s.SetValidatorInfo(valInfo)

	// Send coins back to UnbondTo outputs
	accounts, err := s.GetOrMakeAccounts(nil, valInfo.UnbondTo)
	if err != nil {
		panic("Couldn't get or make unbondTo accounts")
	}
	s.AdjustByOutputs(accounts, valInfo.UnbondTo)
	s.UpdateAccounts(accounts)

	// Remove validator from UnbondingValidators
	_, removed := s.UnbondingValidators.Remove(val.Address)
	if !removed {
		panic("Couldn't remove validator for release")
	}
}

func (s *State) destroyValidator(val *Validator) {
	// Update validatorInfo
	valInfo := s.GetValidatorInfo(val.Address)
	if valInfo == nil {
		panic("Couldn't find validatorInfo for release")
	}
	valInfo.DestroyedHeight = s.LastBlockHeight + 1
	valInfo.DestroyedAmount = val.VotingPower
	s.SetValidatorInfo(valInfo)

	// Remove validator
	_, removed := s.BondedValidators.Remove(val.Address)
	if !removed {
		_, removed := s.UnbondingValidators.Remove(val.Address)
		if !removed {
			panic("Couldn't remove validator for destruction")
		}
	}

}

// NOTE: If an error occurs during block execution, state will be left
// at an invalid state.  Copy the state before calling AppendBlock!
func (s *State) AppendBlock(block *types.Block, blockPartsHeader types.PartSetHeader) error {
	err := s.appendBlock(block, blockPartsHeader)
	if err != nil {
		return err
	}
	// State.Hash should match block.StateHash
	stateHash := s.Hash()
	if !bytes.Equal(stateHash, block.StateHash) {
		return Errorf("Invalid state hash. Expected %X, got %X",
			stateHash, block.StateHash)
	}
	return nil
}

// Mutates the block in place and updates it with new state hash.
func (s *State) SetBlockStateHash(block *types.Block) error {
	sCopy := s.Copy()
	err := sCopy.appendBlock(block, types.PartSetHeader{})
	if err != nil {
		return err
	}
	// Set block.StateHash
	block.StateHash = sCopy.Hash()
	return nil
}

// Appends the block, does not check block.StateHash
// NOTE: If an error occurs during block execution, state will be left
// at an invalid state.  Copy the state before calling appendBlock!
func (s *State) appendBlock(block *types.Block, blockPartsHeader types.PartSetHeader) error {
	// Basic block validation.
	err := block.ValidateBasic(s.LastBlockHeight, s.LastBlockHash, s.LastBlockParts, s.LastBlockTime)
	if err != nil {
		return err
	}

	// Validate block Validation.
	if block.Height == 1 {
		if len(block.Validation.Commits) != 0 {
			return errors.New("Block at height 1 (first block) should have no Validation commits")
		}
	} else {
		if uint(len(block.Validation.Commits)) != s.LastBondedValidators.Size() {
			return errors.New(Fmt("Invalid block validation size. Expected %v, got %v",
				s.LastBondedValidators.Size(), len(block.Validation.Commits)))
		}
		var sumVotingPower uint64
		s.LastBondedValidators.Iterate(func(index uint, val *Validator) bool {
			commit := block.Validation.Commits[index]
			if commit.IsZero() {
				return false
			} else {
				vote := &types.Vote{
					Height:     block.Height - 1,
					Round:      commit.Round,
					Type:       types.VoteTypeCommit,
					BlockHash:  block.LastBlockHash,
					BlockParts: block.LastBlockParts,
				}
				if val.PubKey.VerifyBytes(account.SignBytes(vote), commit.Signature) {
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
	for i, commit := range block.Validation.Commits {
		if commit.IsZero() {
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

	// Commit each tx
	for _, tx := range block.Data.Txs {
		err := s.ExecTx(tx, true)
		if err != nil {
			return InvalidTxError{tx, err}
		}
	}

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

// The returned Account is a copy, so mutating it
// has no side effects.
func (s *State) GetAccount(address []byte) *account.Account {
	_, acc := s.accounts.Get(address)
	if acc == nil {
		return nil
	}
	return acc.(*account.Account).Copy()
}

// The returned Account is a copy, so mutating it
// has no side effects.
func (s *State) GetAccounts() merkle.Tree {
	return s.accounts.Copy()
}

// The account is copied before setting, so mutating it
// afterwards has no side effects.
func (s *State) UpdateAccount(account *account.Account) {
	s.accounts.Set(account.Address, account.Copy())
}

// The accounts are copied before setting, so mutating it
// afterwards has no side effects.
func (s *State) UpdateAccounts(accounts map[string]*account.Account) {
	for _, acc := range accounts {
		s.accounts.Set(acc.Address, acc.Copy())
	}
}

func (s *State) RemoveAccount(address []byte) bool {
	_, removed := s.accounts.Remove(address)
	return removed
}

// The returned ValidatorInfo is a copy, so mutating it
// has no side effects.
func (s *State) GetValidatorInfo(address []byte) *ValidatorInfo {
	_, valInfo := s.validatorInfos.Get(address)
	if valInfo == nil {
		return nil
	}
	return valInfo.(*ValidatorInfo).Copy()
}

// Returns false if new, true if updated.
// The valInfo is copied before setting, so mutating it
// afterwards has no side effects.
func (s *State) SetValidatorInfo(valInfo *ValidatorInfo) (updated bool) {
	return s.validatorInfos.Set(valInfo.Address, valInfo.Copy())
}

// Returns a hash that represents the state data,
// excluding Last*
func (s *State) Hash() []byte {
	hashables := []merkle.Hashable{
		s.BondedValidators,
		s.UnbondingValidators,
		s.accounts,
		s.validatorInfos,
	}
	return merkle.HashFromHashables(hashables)
}
